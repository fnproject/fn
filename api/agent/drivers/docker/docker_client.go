// +build go1.7

package docker

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"golang.org/x/time/rate"
)

type dockerClient interface {
	// Each of these are github.com/fsouza/go-dockerclient methods

	AttachToContainerNonBlocking(ctx context.Context, opts docker.AttachToContainerOptions) (docker.CloseWaiter, error)
	WaitContainerWithContext(id string, ctx context.Context) (int, error)
	StartContainerWithContext(id string, hostConfig *docker.HostConfig, ctx context.Context) error
	KillContainer(opts docker.KillContainerOptions) error
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
	PauseContainer(id string, ctx context.Context) error
	UnpauseContainer(id string, ctx context.Context) error
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	InspectImage(ctx context.Context, name string) (*docker.Image, error)
	ListImages(opts docker.ListImagesOptions) ([]docker.APIImages, error)
	RemoveImage(id string, opts docker.RemoveImageOptions) error
	Stats(opts docker.StatsOptions) error
	Info(ctx context.Context) (*docker.DockerInfo, error)
	LoadImages(ctx context.Context, filePath string) error
	ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error)
	AddEventListener(ctx context.Context) (chan *docker.APIEvents, error)
	RemoveEventListener(ctx context.Context, listener chan *docker.APIEvents) error
}

// TODO: switch to github.com/docker/engine-api
func newClient(ctx context.Context) dockerClient {
	// TODO this was much easier, don't need special settings at the moment
	// docker, err := docker.NewClient(conf.Docker)
	client, err := docker.NewClientFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't create docker client")
	}

	if err := client.Ping(); err != nil {
		logrus.WithError(err).Fatal("couldn't connect to docker daemon")
	}

	wrap := &dockerWrap{docker: client}
	go wrap.listenEventLoop(ctx)
	return wrap
}

type dockerWrap struct {
	docker *docker.Client
}

var (
	apiNameKey     = common.MakeKey("api_name")
	apiStatusKey   = common.MakeKey("api_status")
	exitStatusKey  = common.MakeKey("exit_status")
	eventActionKey = common.MakeKey("event_action")
	eventTypeKey   = common.MakeKey("event_type")

	dockerExitMeasure = common.MakeMeasure("docker_exits", "docker exit counts", "")

	// WARNING: this metric reports total latency per *wrapper* call, which will add up multiple retry latencies per wrapper call.
	dockerLatencyMeasure = common.MakeMeasure("docker_api_latency", "Docker wrapper latency", "msecs")

	dockerEventsMeasure = common.MakeMeasure("docker_events", "docker events", "")

	imageCleanerBusyImgCount = common.MakeMeasure("image_cleaner_busy_img_count", "image cleaner busy image count", "")
	imageCleanerBusyImgSize  = common.MakeMeasure("image_cleaner_busy_img_size", "image cleaner busy image total size", "By")
	imageCleanerIdleImgCount = common.MakeMeasure("image_cleaner_idle_img_count", "image cleaner idle image count", "")
	imageCleanerIdleImgSize  = common.MakeMeasure("image_cleaner_idle_img_size", "image cleaner idle image total size", "By")
	imageCleanerMaxImgSize   = common.MakeMeasure("image_cleaner_max_img_size", "image cleaner image max size", "By")

	dockerInstanceId = common.MakeMeasure("docker_instance_id", "docker instance id", "")
)

func RecordInstanceId(ctx context.Context, id string) {
	h := fnv.New64()
	h.Write([]byte(id))
	hid := int64(h.Sum64())
	stats.Record(ctx, dockerInstanceId.M(hid))
}

func RecordImageCleanerStats(ctx context.Context, sample *ImageCacherStats) {
	stats.Record(ctx, imageCleanerBusyImgCount.M(int64(sample.BusyImgCount)))
	stats.Record(ctx, imageCleanerBusyImgSize.M(int64(sample.BusyImgTotalSize)))
	stats.Record(ctx, imageCleanerIdleImgCount.M(int64(sample.IdleImgCount)))
	stats.Record(ctx, imageCleanerIdleImgSize.M(int64(sample.IdleImgTotalSize)))
	stats.Record(ctx, imageCleanerMaxImgSize.M(int64(sample.MaxImgTotalSize)))
}

// listenEventLoop listens for docker events and reconnects if necessary
func (d *dockerWrap) listenEventLoop(ctx context.Context) {
	limiter := rate.NewLimiter(2.0, 1)
	for limiter.Wait(ctx) == nil {
		err := d.listenEvents(ctx)
		if err != nil {
			logrus.WithError(err).Error("listenEvents failed, will retry...")
		}
	}
}

// listenEvents registers an event listener to docker to stream docker events
// and records these in stats.
func (d *dockerWrap) listenEvents(ctx context.Context) error {
	listener, err := d.AddEventListener(ctx)
	if err != nil {
		return err
	}

	defer d.RemoveEventListener(ctx, listener)

	for {
		select {
		case ev := <-listener:
			if ev == nil {
				return errors.New("event listener closed")
			}

			ctx, err := tag.New(context.Background(),
				tag.Upsert(eventActionKey, ev.Action),
				tag.Upsert(eventTypeKey, ev.Type),
			)
			if err != nil {
				logrus.WithError(err).Fatalf("cannot add event tags %v=%v %v=%v",
					eventActionKey, ev.Action,
					eventTypeKey, ev.Type,
				)
			}

			stats.Record(ctx, dockerEventsMeasure.M(0))
		case <-ctx.Done():
			return nil
		}
	}
}

// Create a span/tracker with required context tags
func makeTracker(ctx context.Context, name string) (context.Context, func(error)) {
	ctx, err := tag.New(ctx, tag.Upsert(apiNameKey, name))
	if err != nil {
		logrus.WithError(err).Fatalf("cannot add tag %v=%v", apiNameKey, name)
	}

	// It would have been nice to pull the latency (end-start) elapsed time
	// from Spans but this is hidden from us, so we have to call time.Now()
	// twice ourselves.
	ctx, span := trace.StartSpan(ctx, name)
	start := time.Now()

	return ctx, func(err error) {

		status := "ok"
		if err != nil {
			if derr, ok := err.(*docker.Error); ok {
				status = strconv.FormatInt(int64(derr.Status), 10)
			} else {
				status = "error"
			}
		}

		ctx, err := tag.New(ctx, tag.Upsert(apiStatusKey, status))
		if err != nil {
			logrus.WithError(err).Fatalf("cannot add tag %v=%v", apiStatusKey, status)
		}

		stats.Record(ctx, dockerLatencyMeasure.M(int64(time.Now().Sub(start)/time.Millisecond)))
		span.End()
	}
}

func RecordWaitContainerResult(ctx context.Context, exitCode int) {

	// Tag the metric with error-code or context-cancel/deadline info
	exitStr := fmt.Sprintf("exit_%d", exitCode)
	if exitCode == 0 && ctx.Err() != nil {
		switch ctx.Err() {
		case context.DeadlineExceeded:
			exitStr = "ctx_deadline"
		case context.Canceled:
			exitStr = "ctx_canceled"
		}
	}

	newCtx, err := tag.New(ctx,
		tag.Upsert(apiNameKey, "docker_wait_container"),
		tag.Upsert(exitStatusKey, exitStr),
	)
	if err != nil {
		logrus.WithError(err).Fatalf("cannot add tag %v=%v or tag %v=docker_wait_container", exitStatusKey, exitStr, apiNameKey)
	}
	stats.Record(newCtx, dockerExitMeasure.M(0))
}

// RegisterViews creates and registers views with provided tag keys
func RegisterViews(tagKeys []string, latencyDist []float64) {

	defaultTags := []tag.Key{apiNameKey, apiStatusKey}
	exitTags := []tag.Key{apiNameKey, exitStatusKey}
	eventTags := []tag.Key{eventActionKey, eventTypeKey}

	// add extra tags if not already in default tags for req/resp
	for _, key := range tagKeys {
		if key != "api_name" && key != "api_status" {
			defaultTags = append(defaultTags, common.MakeKey(key))
		}
		if key != "api_name" && key != "exit_status" {
			exitTags = append(exitTags, common.MakeKey(key))
		}
	}

	// docker instance tags
	emptyTags := []tag.Key{}

	err := view.Register(
		common.CreateViewWithTags(dockerExitMeasure, view.Count(), exitTags),
		common.CreateViewWithTags(dockerLatencyMeasure, view.Distribution(latencyDist...), defaultTags),
		common.CreateViewWithTags(dockerEventsMeasure, view.Count(), eventTags),
		common.CreateViewWithTags(imageCleanerBusyImgCount, view.LastValue(), emptyTags),
		common.CreateViewWithTags(imageCleanerBusyImgSize, view.LastValue(), emptyTags),
		common.CreateViewWithTags(imageCleanerIdleImgCount, view.LastValue(), emptyTags),
		common.CreateViewWithTags(imageCleanerIdleImgSize, view.LastValue(), emptyTags),
		common.CreateViewWithTags(imageCleanerMaxImgSize, view.LastValue(), emptyTags),
		common.CreateViewWithTags(dockerInstanceId, view.LastValue(), emptyTags),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}

func (d *dockerWrap) AddEventListener(ctx context.Context) (listen chan *docker.APIEvents, err error) {
	ctx, closer := makeTracker(ctx, "docker_add_event_listener")
	defer func() { closer(err) }()

	listen = make(chan *docker.APIEvents)
	err = d.docker.AddEventListener(listen)
	if err != nil {
		return nil, err
	}
	return listen, nil
}

func (d *dockerWrap) RemoveEventListener(ctx context.Context, listener chan *docker.APIEvents) (err error) {
	_, closer := makeTracker(ctx, "docker_remove_event_listener")
	defer func() { closer(err) }()
	err = d.docker.RemoveEventListener(listener)
	return err
}

func (d *dockerWrap) ListContainers(opts docker.ListContainersOptions) (containers []docker.APIContainers, err error) {
	_, closer := makeTracker(opts.Context, "docker_list_containers")
	defer func() { closer(err) }()
	containers, err = d.docker.ListContainers(opts)
	return containers, err
}

func (d *dockerWrap) LoadImages(ctx context.Context, filePath string) (err error) {
	ctx, closer := makeTracker(ctx, "docker_load_images")
	defer func() { closer(err) }()

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return err
	}
	defer file.Close()

	// No retries here. LoadImage is typically called at startup and we fail/timeout
	// at first attempt.
	err = d.docker.LoadImage(docker.LoadImageOptions{
		InputStream: file,
		Context:     ctx,
	})
	return err
}

func (d *dockerWrap) ListImages(opts docker.ListImagesOptions) (images []docker.APIImages, err error) {
	_, closer := makeTracker(opts.Context, "docker_list_images")
	defer func() { closer(err) }()
	images, err = d.docker.ListImages(opts)
	return images, err
}

func (d *dockerWrap) Info(ctx context.Context) (info *docker.DockerInfo, err error) {
	_, closer := makeTracker(ctx, "docker_info")
	defer func() { closer(err) }()
	info, err = d.docker.Info()
	return info, err
}

func (d *dockerWrap) AttachToContainerNonBlocking(ctx context.Context, opts docker.AttachToContainerOptions) (w docker.CloseWaiter, err error) {
	_, closer := makeTracker(ctx, "docker_attach_container")
	defer func() { closer(err) }()
	w, err = d.docker.AttachToContainerNonBlocking(opts)
	return w, err
}

func (d *dockerWrap) WaitContainerWithContext(id string, ctx context.Context) (code int, err error) {
	ctx, closer := makeTracker(ctx, "docker_wait_container")
	defer func() { closer(err) }()
	code, err = d.docker.WaitContainerWithContext(id, ctx)
	return code, err
}

func (d *dockerWrap) StartContainerWithContext(id string, hostConfig *docker.HostConfig, ctx context.Context) (err error) {
	ctx, closer := makeTracker(ctx, "docker_start_container")
	defer func() { closer(err) }()
	err = d.docker.StartContainerWithContext(id, hostConfig, ctx)
	return err
}

func (d *dockerWrap) CreateContainer(opts docker.CreateContainerOptions) (c *docker.Container, err error) {
	_, closer := makeTracker(opts.Context, "docker_create_container")
	defer func() { closer(err) }()
	c, err = d.docker.CreateContainer(opts)
	return c, err
}

func (d *dockerWrap) KillContainer(opts docker.KillContainerOptions) (err error) {
	_, closer := makeTracker(opts.Context, "docker_kill_container")
	defer func() { closer(err) }()
	err = d.docker.KillContainer(opts)
	return err
}

func (d *dockerWrap) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) (err error) {
	_, closer := makeTracker(opts.Context, "docker_pull_image")
	defer func() { closer(err) }()
	err = d.docker.PullImage(opts, auth)
	return err
}

func (d *dockerWrap) RemoveImage(image string, opts docker.RemoveImageOptions) (err error) {
	_, closer := makeTracker(opts.Context, "docker_remove_image")
	defer func() { closer(err) }()
	err = d.docker.RemoveImageExtended(image, opts)
	return err
}

func (d *dockerWrap) RemoveContainer(opts docker.RemoveContainerOptions) (err error) {
	_, closer := makeTracker(opts.Context, "docker_remove_container")
	defer func() { closer(err) }()
	err = d.docker.RemoveContainer(opts)
	return err
}

func (d *dockerWrap) PauseContainer(id string, ctx context.Context) (err error) {
	_, closer := makeTracker(ctx, "docker_pause_container")
	defer func() { closer(err) }()
	err = d.docker.PauseContainer(id)
	return err
}

func (d *dockerWrap) UnpauseContainer(id string, ctx context.Context) (err error) {
	_, closer := makeTracker(ctx, "docker_unpause_container")
	defer func() { closer(err) }()
	err = d.docker.UnpauseContainer(id)
	return err
}

func (d *dockerWrap) InspectImage(ctx context.Context, name string) (img *docker.Image, err error) {
	_, closer := makeTracker(ctx, "docker_inspect_image")
	defer func() { closer(err) }()
	img, err = d.docker.InspectImage(name)
	return img, err
}

func (d *dockerWrap) Stats(opts docker.StatsOptions) (err error) {
	_, closer := makeTracker(opts.Context, "docker_stats")
	defer func() { closer(err) }()
	err = d.docker.Stats(opts)
	return err
}
