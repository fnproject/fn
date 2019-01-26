// +build go1.7

package docker

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
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

// wrap docker client calls so we can retry 500s, kind of sucks but fsouza doesn't
// bake in retries we can use internally, could contribute it at some point, would
// be much more convenient if we didn't have to do this, but it's better than ad hoc retries.
// also adds timeouts to many operations, varying by operation
// TODO could generate this, maybe not worth it, may not change often
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
	DiskUsage(opts docker.DiskUsageOptions) (*docker.DiskUsage, error)
	LoadImages(ctx context.Context, filePath string) error
	ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error)
	ListNetworks(ctx context.Context) ([]docker.Network, error)
	AddEventListener(ctx context.Context) (chan *docker.APIEvents, error)
	RemoveEventListener(ctx context.Context, listener chan *docker.APIEvents) error
	InspectContainer(ctx context.Context, id string) (*docker.Container, error)
}

// TODO: switch to github.com/docker/engine-api
func newClient(ctx context.Context, maxRetries uint64) dockerClient {
	// TODO this was much easier, don't need special settings at the moment
	// docker, err := docker.NewClient(conf.Docker)
	client, err := docker.NewClientFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't create docker client")
	}

	if err := client.Ping(); err != nil {
		logrus.WithError(err).Fatal("couldn't connect to docker daemon")
	}

	// punch in default if not set
	if maxRetries == 0 {
		maxRetries = 10
	}

	wrap := &dockerWrap{docker: client, maxRetries: maxRetries}
	go wrap.listenEventLoop(ctx)
	return wrap
}

type dockerWrap struct {
	docker     *docker.Client
	maxRetries uint64
}

var (
	apiNameKey     = common.MakeKey("api_name")
	exitStatusKey  = common.MakeKey("exit_status")
	eventActionKey = common.MakeKey("event_action")
	eventTypeKey   = common.MakeKey("event_type")

	dockerRetriesMeasure = common.MakeMeasure("docker_api_retries", "docker api retries", "")
	dockerTimeoutMeasure = common.MakeMeasure("docker_api_timeout", "docker api timeouts", "")
	dockerErrorMeasure   = common.MakeMeasure("docker_api_error", "docker api errors", "")
	dockerExitMeasure    = common.MakeMeasure("docker_exits", "docker exit counts", "")

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
func makeTracker(ctx context.Context, name string) (context.Context, func()) {
	ctx, err := tag.New(ctx, tag.Upsert(apiNameKey, name))
	if err != nil {
		logrus.WithError(err).Fatalf("cannot add tag %v=%v", apiNameKey, name)
	}

	// It would have been nice to pull the latency (end-start) elapsed time
	// from Spans but this is hidden from us, so we have to call time.Now()
	// twice ourselves.
	ctx, span := trace.StartSpan(ctx, name)
	start := time.Now()

	return ctx, func() {
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

	defaultTags := []tag.Key{apiNameKey}
	exitTags := []tag.Key{apiNameKey, exitStatusKey}
	eventTags := []tag.Key{eventActionKey, eventTypeKey}

	// add extra tags if not already in default tags for req/resp
	for _, key := range tagKeys {
		if key != "api_name" {
			defaultTags = append(defaultTags, common.MakeKey(key))
		}
		if key != "api_name" && key != "exit_status" {
			exitTags = append(exitTags, common.MakeKey(key))
		}
	}

	// docker instance tags
	emptyTags := []tag.Key{}

	err := view.Register(
		common.CreateViewWithTags(dockerRetriesMeasure, view.Sum(), defaultTags),
		common.CreateViewWithTags(dockerTimeoutMeasure, view.Count(), defaultTags),
		common.CreateViewWithTags(dockerErrorMeasure, view.Count(), defaultTags),
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

func (d *dockerWrap) retry(ctx context.Context, f func() error) error {
	var i uint64
	var err error
	defer func() { stats.Record(ctx, dockerRetriesMeasure.M(int64(i))) }()

	logger := common.Logger(ctx)
	var b common.Backoff
	// 10 retries w/o change to backoff is ~13s if ops take ~0 time
	for ; i < d.maxRetries; i++ {
		select {
		case <-ctx.Done():
			stats.Record(ctx, dockerTimeoutMeasure.M(0))
			logger.WithError(ctx.Err()).Warnf("docker call timed out")
			return ctx.Err()
		default:
		}

		err = filter(ctx, f())
		if common.IsTemporary(err) || isDocker50x(err) {
			logger.WithError(err).Warn("docker temporary error, retrying")
			b.Sleep(ctx)
			continue
		}
		if err != nil {
			stats.Record(ctx, dockerErrorMeasure.M(0))
		}
		return err
	}
	return err // TODO could return context.DeadlineExceeded which ~makes sense
}

func isDocker50x(err error) bool {
	derr, ok := err.(*docker.Error)
	return ok && derr.Status >= 500
}

// implement common.Temporary()
type temporary struct {
	error
}

func (t *temporary) Temporary() bool { return true }

func temp(err error) error {
	return &temporary{err}
}

// some 500s are totally cool
func filter(ctx context.Context, err error) error {
	log := common.Logger(ctx)
	// "API error (500): {\"message\":\"service endpoint with name task-57d722ecdecb9e7be16aff17 already exists\"}\n" -> ok since container exists
	switch {
	default:
		return err
	case err == nil:
		return err
	case strings.Contains(err.Error(), "service endpoint with name"):
	}
	log.WithError(err).Warn("filtering error")
	return nil
}

func filterNoSuchContainer(ctx context.Context, err error) error {
	log := common.Logger(ctx)
	if err == nil {
		return nil
	}
	_, containerNotFound := err.(*docker.NoSuchContainer)
	dockerErr, ok := err.(*docker.Error)
	if containerNotFound || (ok && dockerErr.Status == 404) {
		log.WithError(err).Info("filtering error")
		return nil
	}
	return err
}

func (d *dockerWrap) AddEventListener(ctx context.Context) (chan *docker.APIEvents, error) {
	ctx, closer := makeTracker(ctx, "docker_add_event_listener")
	defer closer()

	listen := make(chan *docker.APIEvents)
	err := d.docker.AddEventListener(listen)
	if err != nil {
		return nil, err
	}
	return listen, nil
}

func (d *dockerWrap) RemoveEventListener(ctx context.Context, listener chan *docker.APIEvents) error {
	ctx, closer := makeTracker(ctx, "docker_remove_event_listener")
	defer closer()

	return d.docker.RemoveEventListener(listener)
}

func (d *dockerWrap) ListContainers(opts docker.ListContainersOptions) (containers []docker.APIContainers, err error) {
	ctx, closer := makeTracker(opts.Context, "docker_list_containers")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "ListContainers"})
	err = d.retry(ctx, func() error {
		containers, err = d.docker.ListContainers(opts)
		return err
	})

	return containers, err
}

func (d *dockerWrap) ListNetworks(ctx context.Context) (networks []docker.Network, err error) {
	ctx, closer := makeTracker(ctx, "docker_list_networks")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "ListNetworks"})
	err = d.retry(ctx, func() error {
		networks, err = d.docker.ListNetworks()
		return err
	})

	return networks, err
}

func (d *dockerWrap) LoadImages(ctx context.Context, filePath string) error {
	ctx, closer := makeTracker(ctx, "docker_load_images")
	defer closer()

	file, err := os.Open(filepath.Clean(filePath))
	if err != nil {
		return err
	}
	defer file.Close()

	// No retries here. LoadImage is typically called at startup and we fail/timeout
	// at first attempt.
	return d.docker.LoadImage(docker.LoadImageOptions{
		InputStream: file,
		Context:     ctx,
	})
}

func (d *dockerWrap) ListImages(opts docker.ListImagesOptions) (imgs []docker.APIImages, err error) {
	ctx, closer := makeTracker(opts.Context, "docker_list_images")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "ListImages"})
	err = d.retry(ctx, func() error {
		imgs, err = d.docker.ListImages(opts)
		return err
	})

	return imgs, err
}

func (d *dockerWrap) Info(ctx context.Context) (info *docker.DockerInfo, err error) {
	// NOTE: we're not very responsible and prometheus wasn't loved as a child, this
	// threads through directly down to the docker call, skipping retires, so that we
	// don't have to add tags / tracing / logger to the bare context handed to the one
	// place this is called in initialization that has no context to report consistent
	// stats like everything else in here. tl;dr this works, just don't use it for anything else.
	return d.docker.Info()
}

func (d *dockerWrap) AttachToContainerNonBlocking(ctx context.Context, opts docker.AttachToContainerOptions) (docker.CloseWaiter, error) {
	ctx, closer := makeTracker(ctx, "docker_attach_container")
	defer closer()

	return d.docker.AttachToContainerNonBlocking(opts)
}

func (d *dockerWrap) WaitContainerWithContext(id string, ctx context.Context) (code int, err error) {
	ctx, closer := makeTracker(ctx, "docker_wait_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "WaitContainer"})
	err = d.retry(ctx, func() error {
		code, err = d.docker.WaitContainerWithContext(id, ctx)
		return err
	})
	return code, filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) StartContainerWithContext(id string, hostConfig *docker.HostConfig, ctx context.Context) (err error) {
	ctx, closer := makeTracker(ctx, "docker_start_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "StartContainer"})
	err = d.retry(ctx, func() error {
		err = d.docker.StartContainerWithContext(id, hostConfig, ctx)
		if _, ok := err.(*docker.NoSuchContainer); ok {
			// for some reason create will sometimes return successfully then say no such container here. wtf. so just retry like normal
			return temp(err)
		}
		return err
	})
	return err
}

func (d *dockerWrap) CreateContainer(opts docker.CreateContainerOptions) (c *docker.Container, err error) {
	ctx, closer := makeTracker(opts.Context, "docker_create_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "CreateContainer"})
	err = d.retry(ctx, func() error {
		c, err = d.docker.CreateContainer(opts)
		return err
	})
	return c, err
}

func (d *dockerWrap) KillContainer(opts docker.KillContainerOptions) (err error) {
	ctx, closer := makeTracker(opts.Context, "docker_kill_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "KillContainer"})
	err = d.retry(ctx, func() error {
		err = d.docker.KillContainer(opts)
		return err
	})
	return err
}

func (d *dockerWrap) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) (err error) {
	ctx, closer := makeTracker(opts.Context, "docker_pull_image")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "PullImage"})
	err = d.retry(ctx, func() error {
		err = d.docker.PullImage(opts, auth)
		return err
	})
	return err
}

func (d *dockerWrap) RemoveImage(image string, opts docker.RemoveImageOptions) (err error) {
	ctx, closer := makeTracker(opts.Context, "docker_remove_image")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "RemoveImage"})
	err = d.retry(ctx, func() error {
		err = d.docker.RemoveImageExtended(image, opts)
		return err
	})
	return err

}

func (d *dockerWrap) RemoveContainer(opts docker.RemoveContainerOptions) (err error) {
	ctx, closer := makeTracker(opts.Context, "docker_remove_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "RemoveContainer"})
	err = d.retry(ctx, func() error {
		err = d.docker.RemoveContainer(opts)
		return err
	})
	return filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) PauseContainer(id string, ctx context.Context) (err error) {
	ctx, closer := makeTracker(ctx, "docker_pause_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "PauseContainer"})
	err = d.retry(ctx, func() error {
		err = d.docker.PauseContainer(id)
		return err
	})
	return filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) UnpauseContainer(id string, ctx context.Context) (err error) {
	ctx, closer := makeTracker(ctx, "docker_unpause_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "UnpauseContainer"})
	err = d.retry(ctx, func() error {
		err = d.docker.UnpauseContainer(id)
		return err
	})
	return filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) InspectImage(ctx context.Context, name string) (i *docker.Image, err error) {
	ctx, closer := makeTracker(ctx, "docker_inspect_image")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "InspectImage"})
	err = d.retry(ctx, func() error {
		i, err = d.docker.InspectImage(name)
		return err
	})
	return i, err
}

func (d *dockerWrap) InspectContainer(ctx context.Context, id string) (c *docker.Container, err error) {
	ctx, closer := makeTracker(ctx, "docker_inspect_container")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "InspectContainer"})
	err = d.retry(ctx, func() error {
		c, err = d.docker.InspectContainerWithContext(id, ctx)
		return err
	})
	return c, err
}

func (d *dockerWrap) Stats(opts docker.StatsOptions) (err error) {
	// we can't retry this one this way since the callee closes the
	// stats chan, need a fancier retry mechanism where we can swap out
	// channels, but stats isn't crucial so... be lazy for now
	return d.docker.Stats(opts)

	//err = d.retry(func() error {
	//err = d.docker.Stats(opts)
	//return err
	//})
	//return err
}

func (d *dockerWrap) DiskUsage(opts docker.DiskUsageOptions) (du *docker.DiskUsage, err error) {
	ctx, closer := makeTracker(opts.Context, "docker_disk_usage")
	defer closer()

	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"docker_cmd": "DiskUsage"})
	err = d.retry(ctx, func() error {
		du, err = d.docker.DiskUsage(opts)
		return err
	})
	return du, err
}
