// +build go1.7

// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/treeder/functions/api/runner/common"
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

	AttachToContainerNonBlocking(opts docker.AttachToContainerOptions) (docker.CloseWaiter, error)
	WaitContainerWithContext(id string, ctx context.Context) (int, error)
	StartContainerWithContext(id string, hostConfig *docker.HostConfig, ctx context.Context) error
	CreateContainer(opts docker.CreateContainerOptions) (*docker.Container, error)
	RemoveContainer(opts docker.RemoveContainerOptions) error
	PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error
	InspectImage(name string) (*docker.Image, error)
	InspectContainer(id string) (*docker.Container, error)
	StopContainer(id string, timeout uint) error
	Stats(opts docker.StatsOptions) error
}

// TODO: switch to github.com/docker/engine-api
func newClient(env *common.Environment) dockerClient {
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

	return &dockerWrap{client, clientNoTimeout, env}
}

type dockerWrap struct {
	docker          *docker.Client
	dockerNoTimeout *docker.Client
	*common.Environment
}

func (d *dockerWrap) retry(ctx context.Context, f func() error) error {
	var b common.Backoff
	for {
		select {
		case <-ctx.Done():
			d.Inc("task", "fail.docker", 1, 1)
			logrus.WithError(ctx.Err()).Warnf("retrying on docker errors timed out, restart docker or rotate this instance?")
			return ctx.Err()
		default:
		}

		err := filter(f())
		if common.IsTemporary(err) || isDocker50x(err) {
			logrus.WithError(err).Warn("docker temporary error, retrying")
			b.Sleep()
			d.Inc("task", "error.docker", 1, 1)
			continue
		}
		if err != nil {
			d.Inc("task", "error.docker", 1, 1)
		}
		return err
	}
}

func isDocker50x(err error) bool {
	derr, ok := err.(*docker.Error)
	return ok && derr.Status >= 500
}

func containerConfigError(err error) error {
	derr, ok := err.(*docker.Error)
	if ok && derr.Status == 400 {
		// derr.Message is a JSON response from docker, which has a "message" field we want to extract if possible.
		var v struct {
			Msg string `json:"message"`
		}

		err := json.Unmarshal([]byte(derr.Message), &v)
		if err != nil {
			// If message was not valid JSON, the raw body is still better than nothing.
			return fmt.Errorf("%s", derr.Message)
		}
		return fmt.Errorf("%s", v.Msg)
	}

	return nil
}

type temporary struct {
	error
}

func (t *temporary) Temporary() bool { return true }

func temp(err error) error {
	return &temporary{err}
}

// some 500s are totally cool
func filter(err error) error {
	// "API error (500): {\"message\":\"service endpoint with name task-57d722ecdecb9e7be16aff17 already exists\"}\n" -> ok since container exists
	switch {
	default:
		return err
	case err == nil:
		return err
	case strings.Contains(err.Error(), "service endpoint with name"):
	}
	logrus.WithError(err).Warn("filtering error")
	return nil
}

func filterNoSuchContainer(err error) error {
	if err == nil {
		return nil
	}
	_, containerNotFound := err.(*docker.NoSuchContainer)
	dockerErr, ok := err.(*docker.Error)
	if containerNotFound || (ok && dockerErr.Status == 404) {
		logrus.WithError(err).Error("filtering error")
		return nil
	}
	return err
}

func filterNotRunning(err error) error {
	if err == nil {
		return nil
	}

	_, containerNotRunning := err.(*docker.ContainerNotRunning)
	dockerErr, ok := err.(*docker.Error)
	if containerNotRunning || (ok && dockerErr.Status == 304) {
		logrus.WithError(err).Error("filtering error")
		return nil
	}

	return err
}

func (d *dockerWrap) AttachToContainerNonBlocking(opts docker.AttachToContainerOptions) (w docker.CloseWaiter, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
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
	err = d.retry(ctx, func() error {
		code, err = d.dockerNoTimeout.WaitContainerWithContext(id, ctx)
		return err
	})
	return code, filterNoSuchContainer(err)
}

func (d *dockerWrap) StartContainerWithContext(id string, hostConfig *docker.HostConfig, ctx context.Context) (err error) {
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
	err = d.retry(opts.Context, func() error {
		c, err = d.dockerNoTimeout.CreateContainer(opts)
		return err
	})
	return c, err
}

func (d *dockerWrap) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) (err error) {
	err = d.retry(opts.Context, func() error {
		err = d.dockerNoTimeout.PullImage(opts, auth)
		return err
	})
	return err
}

func (d *dockerWrap) RemoveContainer(opts docker.RemoveContainerOptions) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
		err = d.docker.RemoveContainer(opts)
		return err
	})
	return filterNoSuchContainer(err)
}

func (d *dockerWrap) InspectImage(name string) (i *docker.Image, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
		i, err = d.docker.InspectImage(name)
		return err
	})
	return i, err
}

func (d *dockerWrap) InspectContainer(id string) (c *docker.Container, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
		c, err = d.docker.InspectContainer(id)
		return err
	})
	return c, err
}

func (d *dockerWrap) StopContainer(id string, timeout uint) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), retryTimeout)
	defer cancel()
	err = d.retry(ctx, func() error {
		err = d.docker.StopContainer(id, timeout)
		return err
	})
	return filterNotRunning(filterNoSuchContainer(err))
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
