// +build go1.7

package docker

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

const (
	retryTimeout = 10 * time.Minute
	pauseTimeout = 5 * time.Second
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
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
	PauseContainer(id string, ctx context.Context) error
	UnpauseContainer(id string, ctx context.Context) error
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	InspectImage(ctx context.Context, name string) (*docker.Image, error)
	InspectContainerWithContext(container string, ctx context.Context) (*docker.Container, error)
	Stats(opts docker.StatsOptions) error
	Info(ctx context.Context) (*docker.DockerInfo, error)
}

// TODO: switch to github.com/docker/engine-api
func newClient() dockerClient {
	// TODO this was much easier, don't need special settings at the moment
	// docker, err := docker.NewClient(conf.Docker)
	client, err := docker.NewClientFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't create docker client")
	}

	t := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 1 * time.Minute,
		}).Dial,
		TLSClientConfig: &tls.Config{
			ClientSessionCache: tls.NewLRUClientSessionCache(8192),
		},
		TLSHandshakeTimeout:   10 * time.Second,
		MaxIdleConnsPerHost:   512,
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          512,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client.HTTPClient = &http.Client{Transport: t}

	if err := client.Ping(); err != nil {
		logrus.WithError(err).Fatal("couldn't connect to docker daemon")
	}

	client.SetTimeout(120 * time.Second)

	// get 2 clients, one with a small timeout, one with no timeout to use contexts

	clientNoTimeout, err := docker.NewClientFromEnv()
	if err != nil {
		logrus.WithError(err).Fatal("couldn't create other docker client")
	}

	clientNoTimeout.HTTPClient = &http.Client{Transport: t}

	if err := clientNoTimeout.Ping(); err != nil {
		logrus.WithError(err).Fatal("couldn't connect to other docker daemon")
	}

	return &dockerWrap{client, clientNoTimeout}
}

type dockerWrap struct {
	docker          *docker.Client
	dockerNoTimeout *docker.Client
}

func init() {
	// TODO doing this at each call site seems not the intention of the library since measurements
	// need to be created and views registered. doing this up front seems painful but maybe there
	// are benefits?

	// TODO do we have to do this? the measurements will be tagged on the context, will they be propagated
	// or we have to white list them in the view for them to show up? test...
	appKey, err := tag.NewKey("fn_appname")
	if err != nil {
		logrus.Fatal(err)
	}
	pathKey, err := tag.NewKey("fn_path")
	if err != nil {
		logrus.Fatal(err)
	}

	{
		dockerRetriesMeasure, err = stats.Int64("docker_api_retries", "docker api retries", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			"docker_api_retries_sum",
			"number of times we've retried docker API upon failure",
			[]tag.Key{appKey, pathKey},
			dockerRetriesMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := view.Register(v); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		dockerTimeoutMeasure, err = stats.Int64("docker_api_timeout", "docker api timeouts", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			"docker_api_timeout_count",
			"number of times we've timed out calling docker API",
			[]tag.Key{appKey, pathKey},
			dockerTimeoutMeasure,
			view.CountAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := view.Register(v); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		dockerErrorMeasure, err = stats.Int64("docker_api_error", "docker api errors", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			"docker_api_error_count",
			"number of unrecoverable errors from docker API",
			[]tag.Key{appKey, pathKey},
			dockerErrorMeasure,
			view.CountAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := view.Register(v); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		dockerOOMMeasure, err = stats.Int64("docker_oom", "docker oom", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			"docker_oom_count",
			"number of docker container oom",
			[]tag.Key{appKey, pathKey},
			dockerErrorMeasure,
			view.CountAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := view.Register(v); err != nil {
			logrus.Fatal(err)
		}
	}
}

var (
	// TODO it's either this or stats.FindMeasure("string").M() -- this is safer but painful
	dockerRetriesMeasure *stats.Int64Measure
	dockerTimeoutMeasure *stats.Int64Measure
	dockerErrorMeasure   *stats.Int64Measure
	dockerOOMMeasure     *stats.Int64Measure
)

func (d *dockerWrap) retry(ctx context.Context, logger logrus.FieldLogger, f func() error) error {
	var i int
	var err error
	defer func() { stats.Record(ctx, dockerRetriesMeasure.M(int64(i))) }()

	var b common.Backoff
	// 10 retries w/o change to backoff is ~13s if ops take ~0 time
	for ; i < 10; i++ {
		select {
		case <-ctx.Done():
			stats.Record(ctx, dockerTimeoutMeasure.M(1))
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
			stats.Record(ctx, dockerErrorMeasure.M(1))
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
		log.WithError(err).Error("filtering error")
		return nil
	}
	return err
}

func (d *dockerWrap) Info(ctx context.Context) (info *docker.DockerInfo, err error) {
	ctx, span := trace.StartSpan(ctx, "docker_server_version")
	defer span.End()

	logger := common.Logger(ctx).WithField("docker_cmd", "DockerInfo")
	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()
	err = d.retry(ctx, logger, func() error {
		info, err = d.docker.Info()
		return err
	})
	return info, err
}

func (d *dockerWrap) AttachToContainerNonBlocking(ctx context.Context, opts docker.AttachToContainerOptions) (w docker.CloseWaiter, err error) {
	ctx, span := trace.StartSpan(ctx, "docker_attach_container")
	defer span.End()

	logger := common.Logger(ctx).WithField("docker_cmd", "AttachContainer")
	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()
	err = d.retry(ctx, logger, func() error {
		w, err = d.docker.AttachToContainerNonBlocking(opts)
		if err != nil {
			// always retry if attach errors, task is running, we want logs!
			err = temp(err)
		}
		return err
	})
	return w, err
}

func (d *dockerWrap) WaitContainerWithContext(id string, ctx context.Context) (code int, err error) {
	ctx, span := trace.StartSpan(ctx, "docker_wait_container")
	defer span.End()

	logger := common.Logger(ctx).WithField("docker_cmd", "WaitContainer")
	err = d.retry(ctx, logger, func() error {
		code, err = d.dockerNoTimeout.WaitContainerWithContext(id, ctx)
		return err
	})
	return code, filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) StartContainerWithContext(id string, hostConfig *docker.HostConfig, ctx context.Context) (err error) {
	ctx, span := trace.StartSpan(ctx, "docker_start_container")
	defer span.End()

	logger := common.Logger(ctx).WithField("docker_cmd", "StartContainer")
	err = d.retry(ctx, logger, func() error {
		err = d.dockerNoTimeout.StartContainerWithContext(id, hostConfig, ctx)
		if _, ok := err.(*docker.NoSuchContainer); ok {
			// for some reason create will sometimes return successfully then say no such container here. wtf. so just retry like normal
			return temp(err)
		}
		return err
	})
	return err
}

func (d *dockerWrap) CreateContainer(opts docker.CreateContainerOptions) (c *docker.Container, err error) {
	ctx, span := trace.StartSpan(opts.Context, "docker_create_container")
	defer span.End()

	logger := common.Logger(ctx).WithField("docker_cmd", "CreateContainer")
	err = d.retry(ctx, logger, func() error {
		c, err = d.dockerNoTimeout.CreateContainer(opts)
		return err
	})
	return c, err
}

func (d *dockerWrap) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) (err error) {
	ctx, span := trace.StartSpan(opts.Context, "docker_pull_image")
	defer span.End()

	logger := common.Logger(ctx).WithField("docker_cmd", "PullImage")
	err = d.retry(ctx, logger, func() error {
		err = d.dockerNoTimeout.PullImage(opts, auth)
		return err
	})
	return err
}

func (d *dockerWrap) RemoveContainer(opts docker.RemoveContainerOptions) (err error) {
	// extract the span, but do not keep the context, since the enclosing context
	// may be timed out, and we still want to remove the container. TODO in caller? who cares?
	_, span := trace.StartSpan(opts.Context, "docker_remove_container")
	defer span.End()
	ctx := trace.WithSpan(context.Background(), span)

	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()

	logger := common.Logger(ctx).WithField("docker_cmd", "RemoveContainer")
	err = d.retry(ctx, logger, func() error {
		err = d.docker.RemoveContainer(opts)
		return err
	})
	return filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) PauseContainer(id string, ctx context.Context) (err error) {
	_, span := trace.StartSpan(ctx, "docker_pause_container")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, pauseTimeout)
	defer cancel()

	logger := common.Logger(ctx).WithField("docker_cmd", "PauseContainer")
	err = d.retry(ctx, logger, func() error {
		err = d.docker.PauseContainer(id)
		return err
	})
	return filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) UnpauseContainer(id string, ctx context.Context) (err error) {
	_, span := trace.StartSpan(ctx, "docker_unpause_container")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, pauseTimeout)
	defer cancel()

	logger := common.Logger(ctx).WithField("docker_cmd", "UnpauseContainer")
	err = d.retry(ctx, logger, func() error {
		err = d.docker.UnpauseContainer(id)
		return err
	})
	return filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) InspectImage(ctx context.Context, name string) (i *docker.Image, err error) {
	ctx, span := trace.StartSpan(ctx, "docker_inspect_image")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()

	logger := common.Logger(ctx).WithField("docker_cmd", "InspectImage")
	err = d.retry(ctx, logger, func() error {
		i, err = d.docker.InspectImage(name)
		return err
	})
	return i, err
}

func (d *dockerWrap) InspectContainerWithContext(container string, ctx context.Context) (c *docker.Container, err error) {
	ctx, span := trace.StartSpan(ctx, "docker_inspect_container")
	defer span.End()
	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()

	logger := common.Logger(ctx).WithField("docker_cmd", "InspectContainer")
	err = d.retry(ctx, logger, func() error {
		c, err = d.docker.InspectContainerWithContext(container, ctx)
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
