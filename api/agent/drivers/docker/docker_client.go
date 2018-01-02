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
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/sirupsen/logrus"
)

const (
	retryTimeout = 10 * time.Minute
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
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	InspectImage(ctx context.Context, name string) (*docker.Image, error)
	InspectContainerWithContext(container string, ctx context.Context) (*docker.Container, error)
	Stats(opts docker.StatsOptions) error
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

func (d *dockerWrap) retry(ctx context.Context, f func() error) error {
	var i int
	span := opentracing.SpanFromContext(ctx)
	defer func() { span.LogFields(log.Int("docker_call_retries", i)) }()

	logger := common.Logger(ctx)
	var b common.Backoff
	for ; ; i++ {
		select {
		case <-ctx.Done():
			span.LogFields(log.String("task", "fail.docker"))
			logger.WithError(ctx.Err()).Warnf("docker call timed out")
			return ctx.Err()
		default:
		}

		err := filter(ctx, f())
		if common.IsTemporary(err) || isDocker50x(err) {
			logger.WithError(err).Warn("docker temporary error, retrying")
			b.Sleep(ctx)
			span.LogFields(log.String("task", "tmperror.docker"))
			continue
		}
		if err != nil {
			span.LogFields(log.String("task", "error.docker"))
		}
		return err
	}
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

func (d *dockerWrap) AttachToContainerNonBlocking(ctx context.Context, opts docker.AttachToContainerOptions) (w docker.CloseWaiter, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "docker_attach_container")
	defer span.Finish()

	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
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
	span, ctx := opentracing.StartSpanFromContext(ctx, "docker_wait_container")
	defer span.Finish()
	err = d.retry(ctx, func() error {
		code, err = d.dockerNoTimeout.WaitContainerWithContext(id, ctx)
		return err
	})
	return code, filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) StartContainerWithContext(id string, hostConfig *docker.HostConfig, ctx context.Context) (err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "docker_start_container")
	defer span.Finish()
	err = d.retry(ctx, func() error {
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
	span, ctx := opentracing.StartSpanFromContext(opts.Context, "docker_create_container")
	defer span.Finish()
	err = d.retry(ctx, func() error {
		c, err = d.dockerNoTimeout.CreateContainer(opts)
		return err
	})
	return c, err
}

func (d *dockerWrap) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) (err error) {
	span, ctx := opentracing.StartSpanFromContext(opts.Context, "docker_pull_image")
	defer span.Finish()
	err = d.retry(ctx, func() error {
		err = d.dockerNoTimeout.PullImage(opts, auth)
		return err
	})
	return err
}

func (d *dockerWrap) RemoveContainer(opts docker.RemoveContainerOptions) (err error) {
	// extract the span, but do not keep the context, since the enclosing context
	// may be timed out, and we still want to remove the container. TODO in caller? who cares?
	span, _ := opentracing.StartSpanFromContext(opts.Context, "docker_remove_container")
	defer span.Finish()
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
		err = d.docker.RemoveContainer(opts)
		return err
	})
	return filterNoSuchContainer(ctx, err)
}

func (d *dockerWrap) InspectImage(ctx context.Context, name string) (i *docker.Image, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "docker_inspect_image")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
		i, err = d.docker.InspectImage(name)
		return err
	})
	return i, err
}

func (d *dockerWrap) InspectContainerWithContext(container string, ctx context.Context) (c *docker.Container, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "docker_inspect_container")
	defer span.Finish()
	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
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
