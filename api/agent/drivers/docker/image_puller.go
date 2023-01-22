package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"

	"github.com/fsouza/go-dockerclient"
)

// ImagePuller is an abstraction layer to handle concurrent docker-pulls. Docker internally
// does not handle concurrency very well. Only layer-blob pulls have a listener/follow serialization
// leaving manifest/config fetches out. For instance, a single layer image, when merely two docker-pulls
// are initiated at the same time, this causes 9 HTTP-GET requests from docker to the repository where
// 4 extra & unnecessary HTTP GETs are initiated. Below is a simple listener/follower serialization, where
// any new requests are added as listeners to the ongoing docker-pull requests.

type ImagePuller interface {
	PullImage(ctx context.Context, cfg *docker.AuthConfiguration, img, repo, tag string) chan error
	SetRetryPolicy(policy common.BackOffConfig, checker drivers.RetryErrorChecker) error
}

type transfer struct {
	ctx context.Context // oldest context

	key string

	cfg  *docker.AuthConfiguration
	img  string
	repo string
	tag  string

	listeners []chan error
}

type imagePuller struct {
	docker dockerClient

	lock      sync.Mutex
	transfers map[string]*transfer

	// backoff/retry settings
	isRetriable drivers.RetryErrorChecker
	backOffCfg  common.BackOffConfig
}

func NewImagePuller(docker dockerClient) ImagePuller {
	c := imagePuller{
		docker:      docker,
		transfers:   make(map[string]*transfer),
		isRetriable: func(error) (bool, string) { return false, "" },
	}

	return &c
}

func (i *imagePuller) SetRetryPolicy(policy common.BackOffConfig, checker drivers.RetryErrorChecker) error {
	i.isRetriable = checker
	i.backOffCfg = policy
	return nil
}

// newTransfer initiates a new docker-pull if there's no active docker-pull present for the same image.
func (i *imagePuller) newTransfer(ctx context.Context, cfg *docker.AuthConfiguration, img, repo, tag string) chan error {

	key := fmt.Sprintf("%s %s %+v", repo, tag, cfg)

	i.lock.Lock()

	trx, ok := i.transfers[key]
	if !ok {
		trx = &transfer{
			ctx:       ctx,
			key:       key,
			cfg:       cfg,
			img:       img,
			repo:      repo,
			tag:       tag,
			listeners: make([]chan error, 0, 1),
		}
		i.transfers[key] = trx
	}

	errC := make(chan error, 1)
	trx.listeners = append(trx.listeners, errC)

	i.lock.Unlock()

	// First time call for this image/key, start a docker-pull
	if !ok {
		go i.startTransfer(trx)
	}

	return errC
}

func (i *imagePuller) pullWithRetry(trx *transfer) error {
	backoff := common.NewBackOff(i.backOffCfg)
	timer := common.NewTimer(time.Duration(i.backOffCfg.MinDelay) * time.Millisecond)
	defer timer.Stop()

	for {
		err := i.docker.PullImage(docker.PullImageOptions{Repository: trx.repo, Tag: trx.tag, Context: trx.ctx}, *trx.cfg)
		ok, reason := i.isRetriable(err)
		if !ok {
			return err
		}

		delay, ok := backoff.NextBackOff()
		if !ok {
			return err
		}

		timer.Reset(delay)

		select {
		case <-timer.C:
			recordRetry(trx.ctx, "docker_pull_image", reason)
		case <-trx.ctx.Done():
			return trx.ctx.Err()
		}
	}
}

func (i *imagePuller) startTransfer(trx *transfer) {
	var ferr error

	//~~ to remove
	fmt.Printf("~~~~image to pull: %s\n",trx.img)
	err := i.pullWithRetry(trx)
	if err != nil {
		common.Logger(trx.ctx).WithError(err).Info("Failed to pull image")

		// TODO need to inspect for hub or network errors and pick; for now, assume
		// 500 if not a docker error
		msg := err.Error()
		code := http.StatusBadGateway
		if dErr, ok := err.(*docker.Error); ok {
			msg = dockerMsg(dErr)
			if dErr.Status >= 400 && dErr.Status < 500 {
				code = dErr.Status // decap 4xx errors
			}
		}

		err := models.NewAPIError(code, fmt.Errorf("Failed to pull image '%s': %s", trx.img, msg))
		ferr = models.NewFuncError(err)
	}

	i.lock.Lock()
	defer i.lock.Unlock()

	// notify any listeners
	for _, ch := range trx.listeners {
		if ferr != nil {
			ch <- ferr
		}
		close(ch)
	}

	// unregister the docker-pull
	delete(i.transfers, trx.key)
}

func (i *imagePuller) PullImage(ctx context.Context, cfg *docker.AuthConfiguration, img, repo, tag string) chan error {
	return i.newTransfer(ctx, cfg, img, repo, tag)
}

// removes docker err formatting: 'API Error (code) {"message":"..."}'
func dockerMsg(derr *docker.Error) string {
	// derr.Message is a JSON response from docker, which has a "message" field we want to extract if possible.
	// this is pretty lame, but it is what it is
	var v struct {
		Msg string `json:"message"`
	}

	err := json.Unmarshal([]byte(derr.Message), &v)
	if err != nil {
		// If message was not valid JSON, the raw body is still better than nothing.
		return derr.Message
	}
	return v.Msg
}
