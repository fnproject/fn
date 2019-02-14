package docker

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/fsouza/go-dockerclient"

	"github.com/fnproject/fn/api/agent/drivers"
)

type mockClient struct {
	dockerWrap
	listImages    []docker.APIImages
	removedImages []string

	inspectImage    *docker.Image
	inspectImageErr error
}

func (c *mockClient) ListImages(opts docker.ListImagesOptions) ([]docker.APIImages, error) {
	return c.listImages, nil
}
func (c *mockClient) RemoveImage(id string, opts docker.RemoveImageOptions) error {
	c.removedImages = append(c.removedImages, id)
	return nil
}
func (c *mockClient) InspectImage(ctx context.Context, name string) (i *docker.Image, err error) {
	return c.inspectImage, c.inspectImageErr
}

// Basic startup scenario. ListImages() with exempt as well as exceeding capacity images
// should result in image removals.
func TestImageCleaner1(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	dkr := &DockerDriver{
		cancel:   cancel,
		conf:     drivers.Config{},
		docker:   &mockClient{},
		network:  NewDockerNetworks(drivers.Config{}),
		imgCache: NewImageCache([]string{"exempt"}, uint64(1024*1024)),
	}

	defer dkr.Close()

	inner := dkr.imgCache.(*imageCacher)
	mock := dkr.docker.(*mockClient)

	mock.listImages = append(mock.listImages, docker.APIImages{
		ID:       "zoo0",
		RepoTags: []string{"exempt"},
		Size:     512 * 1024,
	})
	mock.listImages = append(mock.listImages, docker.APIImages{
		ID:   "zoo1",
		Size: 512 * 1024,
	})
	mock.listImages = append(mock.listImages, docker.APIImages{
		ID:   "zoo2",
		Size: 512 * 1024,
	})
	mock.listImages = append(mock.listImages, docker.APIImages{
		ID:   "zoo3",
		Size: 512 * 1024,
	})

	go func() {
		syncImageCleaner(ctx, dkr)
		runImageCleaner(ctx, dkr)
	}()

	select {
	case <-time.After(5 * time.Second):
		if len(mock.removedImages) != 2 {
			t.Fatalf("fail cache=%+v removed=%+v", inner, mock.removedImages)
		} else if mock.removedImages[0] != "zoo1" || mock.removedImages[1] != "zoo2" {
			t.Fatalf("fail cache=%+v removed=%+v", inner, mock.removedImages)
		}
	case <-ctx.Done():
		t.Fatalf("ctx timeout cache=%+v removed=%+v", inner, mock.removedImages)
	}

}

// Basic cookie scenario, cookie marks a huge image as busy, then unmark it. Image
// cache should not report IsMaxCapacity() first, but after unmark, it should...
func TestImageCleaner2(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	dkr := &DockerDriver{
		cancel:   cancel,
		conf:     drivers.Config{},
		docker:   &mockClient{},
		network:  NewDockerNetworks(drivers.Config{}),
		imgCache: NewImageCache([]string{}, uint64(1024*1024)),
	}

	defer dkr.Close()

	var output bytes.Buffer
	var errors bytes.Buffer

	inner := dkr.imgCache.(*imageCacher)
	mock := dkr.docker.(*mockClient)

	// huge  image
	mock.inspectImage = &docker.Image{
		ID:   "zoo0",
		Size: 512 * 1024 * 1024,
	}

	task := createTask("test-docker")
	task.output = &output
	task.errors = &errors

	cookie, err := dkr.CreateCookie(ctx, task)
	if err != nil {
		t.Fatal("Couldn't create task cookie")
	}

	shouldPull, err := cookie.ValidateImage(ctx)
	if err != nil || shouldPull {
		t.Fatalf("Couldn't validate image test cache=%+v", inner)
	}

	if inner.IsMaxCapacity() {
		t.Fatalf("should not be max capacity cache=%+v", inner)
	}
	if item := inner.Pop(); item != nil {
		t.Fatalf("should not pop item (busy) cache=%+v item=%+v", inner, item)
	}

	cookie.Close(ctx)

	if !inner.IsMaxCapacity() {
		t.Fatalf("should be max capacity cache=%+v", inner)
	}
	if item := inner.Pop(); item == nil {
		t.Fatalf("should pop item cache=%+v", inner)
	}
}
