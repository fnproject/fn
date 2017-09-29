package service

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
	// Import builders to get the builder function as package function
	. "github.com/docker/cli/internal/test/builders"
)

type fakeClient struct {
	client.Client
	serviceInspectWithRawFunc func(ctx context.Context, serviceID string, options types.ServiceInspectOptions) (swarm.Service, []byte, error)
	serviceUpdateFunc         func(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error)
	serviceListFunc           func(context.Context, types.ServiceListOptions) ([]swarm.Service, error)
	infoFunc                  func(ctx context.Context) (types.Info, error)
}

func (f *fakeClient) NodeList(ctx context.Context, options types.NodeListOptions) ([]swarm.Node, error) {
	return nil, nil
}

func (f *fakeClient) TaskList(ctx context.Context, options types.TaskListOptions) ([]swarm.Task, error) {
	return nil, nil
}

func (f *fakeClient) ServiceInspectWithRaw(ctx context.Context, serviceID string, options types.ServiceInspectOptions) (swarm.Service, []byte, error) {
	if f.serviceInspectWithRawFunc != nil {
		return f.serviceInspectWithRawFunc(ctx, serviceID, options)
	}

	return *Service(ServiceID(serviceID)), []byte{}, nil
}

func (f *fakeClient) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	if f.serviceListFunc != nil {
		return f.serviceListFunc(ctx, options)
	}

	return nil, nil
}

func (f *fakeClient) ServiceUpdate(ctx context.Context, serviceID string, version swarm.Version, service swarm.ServiceSpec, options types.ServiceUpdateOptions) (types.ServiceUpdateResponse, error) {
	if f.serviceUpdateFunc != nil {
		return f.serviceUpdateFunc(ctx, serviceID, version, service, options)
	}

	return types.ServiceUpdateResponse{}, nil
}

func (f *fakeClient) Info(ctx context.Context) (types.Info, error) {
	if f.infoFunc == nil {
		return types.Info{}, nil
	}
	return f.infoFunc(ctx)
}

func newService(id string, name string) swarm.Service {
	return swarm.Service{
		ID:   id,
		Spec: swarm.ServiceSpec{Annotations: swarm.Annotations{Name: name}},
	}
}
