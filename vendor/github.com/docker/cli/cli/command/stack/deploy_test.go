package stack

import (
	"testing"

	"github.com/docker/cli/cli/compose/convert"
	"github.com/docker/cli/internal/test"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestPruneServices(t *testing.T) {
	ctx := context.Background()
	namespace := convert.NewNamespace("foo")
	services := map[string]struct{}{
		"new":  {},
		"keep": {},
	}
	client := &fakeClient{services: []string{objectName("foo", "keep"), objectName("foo", "remove")}}
	dockerCli := test.NewFakeCli(client)

	pruneServices(ctx, dockerCli, namespace, services)
	assert.Equal(t, buildObjectIDs([]string{objectName("foo", "remove")}), client.removedServices)
}

// TestServiceUpdateResolveImageChanged tests that the service's
// image digest is preserved if the image did not change in the compose file
func TestServiceUpdateResolveImageChanged(t *testing.T) {
	namespace := convert.NewNamespace("mystack")

	var (
		receivedOptions types.ServiceUpdateOptions
		receivedService swarm.ServiceSpec
	)

	client := test.NewFakeCli(&fakeClient{
		serviceListFunc: func(options types.ServiceListOptions) ([]swarm.Service, error) {
			return []swarm.Service{
				{
					Spec: swarm.ServiceSpec{
						Annotations: swarm.Annotations{
							Name:   namespace.Name() + "_myservice",
							Labels: map[string]string{"com.docker.stack.image": "foobar:1.2.3"},
						},
						TaskTemplate: swarm.TaskSpec{
							ContainerSpec: &swarm.ContainerSpec{
								Image: "foobar:1.2.3@sha256:deadbeef",
							},
						},
					},
				},
			}, nil
		},
		serviceUpdateFunc: func(serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error) {
			receivedOptions = options
			receivedService = service
			return types.ServiceUpdateResponse{}, nil
		},
	})

	var testcases = []struct {
		image                 string
		expectedQueryRegistry bool
		expectedImage         string
	}{
		// Image not changed
		{
			image: "foobar:1.2.3",
			expectedQueryRegistry: false,
			expectedImage:         "foobar:1.2.3@sha256:deadbeef",
		},
		// Image changed
		{
			image: "foobar:1.2.4",
			expectedQueryRegistry: true,
			expectedImage:         "foobar:1.2.4",
		},
	}

	ctx := context.Background()

	for _, testcase := range testcases {
		t.Logf("Testing image %q", testcase.image)
		spec := map[string]swarm.ServiceSpec{
			"myservice": {
				TaskTemplate: swarm.TaskSpec{
					ContainerSpec: &swarm.ContainerSpec{
						Image: testcase.image,
					},
				},
			},
		}
		err := deployServices(ctx, client, spec, namespace, false, resolveImageChanged)
		assert.NoError(t, err)
		assert.Equal(t, testcase.expectedQueryRegistry, receivedOptions.QueryRegistry)
		assert.Equal(t, testcase.expectedImage, receivedService.TaskTemplate.ContainerSpec.Image)

		receivedService = swarm.ServiceSpec{}
		receivedOptions = types.ServiceUpdateOptions{}
	}
}
