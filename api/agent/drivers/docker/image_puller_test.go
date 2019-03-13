package docker

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"

	"github.com/fsouza/go-dockerclient"
)

type mockClientPuller struct {
	dockerWrap

	numCalls uint64
}

func (c *mockClientPuller) PullImage(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
	time.Sleep(time.Second * 1)
	atomic.AddUint64(&c.numCalls, uint64(1))
	return nil
}

// Lets do concurrent docker-pulls for an image with two different tags. This should results in only
// two calls to docker-pull.
func TestImagePullConcurrent1(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	var cli dockerClient
	mock := mockClientPuller{}
	cli = &mock

	puller := NewImagePuller(drivers.Config{}, cli)

	cfg := docker.AuthConfiguration{}
	img := "foo"
	repo := "zoo"
	tag1 := "1.0.0"
	tag2 := "1.0.1"

	var wg sync.WaitGroup
	wg.Add(20)

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			<-puller.PullImage(ctx, &cfg, img, repo, tag1)
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			<-puller.PullImage(ctx, &cfg, img, repo, tag2)
		}()
	}

	wg.Wait()

	// Should be two docker-pulls
	if mock.numCalls != 2 || ctx.Err() != nil {
		t.Fatalf("fail numOfPulls=%d ctx=%s", mock.numCalls, ctx.Err())
	}
}
