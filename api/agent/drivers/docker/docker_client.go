// +build go1.7

package docker

import (
	"context"
	"hash/fnv"
	"io"
	"strconv"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	realdocker "github.com/docker/docker/client"
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
	// Each of these match github.com/docker/docker/client methods directly
	ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (types.HijackedResponse, error)
	ContainerList(context.Context, types.ContainerListOptions) ([]types.Container, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerStats(ctx context.Context, container string, stream bool) (types.ContainerStats, error)
	ContainerPause(ctx context.Context, container string) error
	ContainerUnpause(ctx context.Context, container string) error
	ContainerRemove(ctx context.Context, container string, options types.ContainerRemoveOptions) error
	ContainerKill(ctx context.Context, container, signal string) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerStart(ctx context.Context, container string, options types.ContainerStartOptions) error
	ContainerWait(ctx context.Context, container string, condition containertypes.WaitCondition) (<-chan containertypes.ContainerWaitOKBody, <-chan error)

	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	Info(context.Context) (types.Info, error)

	ImageRemove(ctx context.Context, image string, options types.ImageRemoveOptions) ([]types.ImageDeleteResponseItem, error)
	ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error)
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)
	ImagePull(ctx context.Context, refStr string, options types.ImagePullOptions) (io.ReadCloser, error)
}

func newClient(ctx context.Context) dockerClient {
	realclient, err := realdocker.NewClientWithOpts(realdocker.FromEnv)
	if err != nil {
		logrus.WithError(err).Fatal("couldn't create docker client")
	}

	// this syncs to the latest possible API based on running docker version and our bindings
	realclient.NegotiateAPIVersion(ctx)

	if _, err := realclient.Ping(ctx); err != nil {
		logrus.WithError(err).Fatal("couldn't connect to docker daemon")
	}

	// TODO this was much easier, don't need special settings at the moment
	// docker, err := docker.NewClient(conf.Docker)
	client, err := docker.NewClientFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't create docker client")
	}

	if err := client.Ping(); err != nil {
		logrus.WithError(err).Fatal("couldn't connect to docker daemon")
	}

	wrap := &dockerWrap{realdocker: realclient, docker: client}
	go wrap.listenEventLoop(ctx)
	return wrap
}

type dockerWrap struct {
	realdocker *realdocker.Client
	docker     *docker.Client
}

var (
	apiNameKey     = common.MakeKey("api_name")
	apiStatusKey   = common.MakeKey("api_status")
	exitStatusKey  = common.MakeKey("exit_status")
	eventActionKey = common.MakeKey("event_action")
	eventTypeKey   = common.MakeKey("event_type")

	dockerRetriesMeasure = common.MakeMeasure("docker_api_retries", "docker api retries", "")
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // this removes the event listener

	listener, errCh := d.realdocker.Events(ctx, types.EventsOptions{})

	for {
		select {
		case err := <-errCh:
			if err == io.EOF { // event stream closed
				return nil
			}
			return err // other error
		case ev := <-listener:

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

// record a retry attempt for the api with status/reason provided
func recordRetry(ctx context.Context, apiName, apiStatus string) {

	ctx, err := tag.New(ctx,
		tag.Upsert(apiNameKey, apiName),
		tag.Upsert(apiStatusKey, apiStatus),
	)
	if err != nil {
		logrus.WithError(err).Fatalf("cannot add tags %v=%v %v=%v", apiNameKey, apiName, apiStatusKey, apiStatus)
	}

	stats.Record(ctx, dockerRetriesMeasure.M(0))
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
			if err == context.Canceled {
				status = "canceled"
			} else if err == context.DeadlineExceeded {
				status = "timeout"
			} else if derr, ok := err.(*docker.Error); ok {
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
		common.CreateViewWithTags(dockerRetriesMeasure, view.Count(), defaultTags),
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

func (d *dockerWrap) ContainerList(ctx context.Context, options types.ContainerListOptions) (containers []types.Container, err error) {
	_, closer := makeTracker(ctx, "docker_list_containers")
	defer func() { closer(err) }()
	containers, err = d.realdocker.ContainerList(ctx, options)
	return containers, err
}

func (d *dockerWrap) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (resp types.ImageLoadResponse, err error) {
	ctx, closer := makeTracker(ctx, "docker_load_images")
	defer func() { closer(err) }()
	resp, err = d.realdocker.ImageLoad(ctx, input, quiet)
	return resp, err
}

func (d *dockerWrap) ImageList(ctx context.Context, options types.ImageListOptions) (images []types.ImageSummary, err error) {
	_, closer := makeTracker(ctx, "docker_list_images")
	defer func() { closer(err) }()
	images, err = d.realdocker.ImageList(ctx, options)
	return images, err
}

func (d *dockerWrap) Info(ctx context.Context) (info types.Info, err error) {
	_, closer := makeTracker(ctx, "docker_info")
	defer func() { closer(err) }()
	info, err = d.realdocker.Info(ctx)
	return info, err
}

func (d *dockerWrap) ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (resp types.HijackedResponse, err error) {
	_, closer := makeTracker(ctx, "docker_attach_container")
	defer func() { closer(err) }()
	resp, err = d.realdocker.ContainerAttach(ctx, container, options)
	return resp, err
}

func (d *dockerWrap) ContainerWait(ctx context.Context, container string, condition containertypes.WaitCondition) (ch <-chan containertypes.ContainerWaitOKBody, err <-chan error) {
	ctx, closer := makeTracker(ctx, "docker_wait_container")
	defer func() { closer(nil) }()
	ch, err = d.realdocker.ContainerWait(ctx, container, condition)
	return ch, err
}

func (d *dockerWrap) ContainerStart(ctx context.Context, container string, options types.ContainerStartOptions) (err error) {
	ctx, closer := makeTracker(ctx, "docker_start_container")
	defer func() { closer(err) }()
	err = d.realdocker.ContainerStart(ctx, container, options)
	return err
}

func (d *dockerWrap) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (c container.ContainerCreateCreatedBody, err error) {
	ctx, closer := makeTracker(ctx, "docker_create_container")
	defer func() { closer(err) }()
	c, err = d.realdocker.ContainerCreate(ctx, config, hostConfig, networkingConfig, containerName)
	return c, err
}

func (d *dockerWrap) ContainerInspect(ctx context.Context, containerID string) (container types.ContainerJSON, err error) {
	ctx, closer := makeTracker(ctx, "docker_create_container")
	defer func() { closer(err) }()
	container, err = d.realdocker.ContainerInspect(ctx, containerID)
	return container, err
}

func (d *dockerWrap) ContainerKill(ctx context.Context, container, signal string) (err error) {
	_, closer := makeTracker(ctx, "docker_kill_container")
	defer func() { closer(err) }()
	err = d.realdocker.ContainerKill(ctx, container, signal)
	return err
}

func (d *dockerWrap) ImagePull(ctx context.Context, refStr string, options types.ImagePullOptions) (resp io.ReadCloser, err error) {
	_, closer := makeTracker(ctx, "docker_pull_image")
	defer func() { closer(err) }()
	resp, err = d.realdocker.ImagePull(ctx, refStr, options)
	return resp, err
}

func (d *dockerWrap) ImageRemove(ctx context.Context, image string, options types.ImageRemoveOptions) (resp []types.ImageDeleteResponseItem, err error) {
	_, closer := makeTracker(ctx, "docker_remove_image")
	defer func() { closer(err) }()
	resp, err = d.realdocker.ImageRemove(ctx, image, options)
	return resp, err
}

func (d *dockerWrap) ContainerRemove(ctx context.Context, container string, options types.ContainerRemoveOptions) (err error) {
	_, closer := makeTracker(ctx, "docker_remove_container")
	defer func() { closer(err) }()
	err = d.realdocker.ContainerRemove(ctx, container, options)
	return err
}

func (d *dockerWrap) ContainerPause(ctx context.Context, container string) (err error) {
	_, closer := makeTracker(ctx, "docker_pause_container")
	defer func() { closer(err) }()
	err = d.realdocker.ContainerUnpause(ctx, container)
	return err
}

func (d *dockerWrap) ContainerUnpause(ctx context.Context, container string) (err error) {
	_, closer := makeTracker(ctx, "docker_unpause_container")
	defer func() { closer(err) }()
	err = d.realdocker.ContainerUnpause(ctx, container)
	return err
}

func (d *dockerWrap) ImageInspectWithRaw(ctx context.Context, imageID string) (image types.ImageInspect, b []byte, err error) {
	_, closer := makeTracker(ctx, "docker_inspect_image")
	defer func() { closer(err) }()
	image, b, err = d.realdocker.ImageInspectWithRaw(ctx, imageID)
	return image, b, err
}

func (d *dockerWrap) ContainerStats(ctx context.Context, containerID string, stream bool) (stats types.ContainerStats, err error) {
	_, closer := makeTracker(ctx, "docker_stats")
	defer func() { closer(err) }()
	stats, err = d.realdocker.ContainerStats(ctx, containerID, stream)
	return stats, err
}
