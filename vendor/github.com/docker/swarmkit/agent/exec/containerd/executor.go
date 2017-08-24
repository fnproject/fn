package containerd

import (
	"os"
	"runtime"

	"github.com/containerd/containerd"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/swarmkit/agent/exec"
	"github.com/docker/swarmkit/agent/secrets"
	"github.com/docker/swarmkit/api"
	"github.com/docker/swarmkit/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type executor struct {
	client           *containerd.Client
	secrets          exec.SecretsManager
	genericResources []*api.GenericResource
}

var _ exec.Executor = &executor{}

// NewExecutor returns an executor using the given containerd control socket
func NewExecutor(sock, namespace string, genericResources []*api.GenericResource) (exec.Executor, error) {
	if namespace == "" {
		return nil, errors.New("A containerd namespace is required")
	}
	client, err := containerd.New(sock, containerd.WithDefaultNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "creating containerd client")
	}

	return &executor{
		client:           client,
		secrets:          secrets.NewManager(),
		genericResources: genericResources,
	}, nil
}

// Describe returns the underlying node description from containerd
func (e *executor) Describe(ctx context.Context) (*api.NodeDescription, error) {
	ctx = log.WithModule(ctx, "containerd")

	hostname := ""
	if hn, err := os.Hostname(); err != nil {
		log.G(ctx).Warnf("Could not get hostname: %v", err)
	} else {
		hostname = hn
	}

	meminfo, err := system.ReadMemInfo()
	if err != nil {
		log.G(ctx).WithError(err).Error("Failed to read meminfo")
		meminfo = &system.MemInfo{}
	}

	description := &api.NodeDescription{
		Hostname: hostname,
		Platform: &api.Platform{
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
		},
		Resources: &api.Resources{
			NanoCPUs:    int64(sysinfo.NumCPU()),
			MemoryBytes: meminfo.MemTotal,
			Generic:     e.genericResources,
		},
	}

	return description, nil
}

func (e *executor) Configure(ctx context.Context, node *api.Node) error {
	return nil
}

// Controller returns a docker container controller.
func (e *executor) Controller(t *api.Task) (exec.Controller, error) {
	ctlr, err := newController(e.client, t, secrets.Restrict(e.secrets, t))
	if err != nil {
		return nil, err
	}

	return ctlr, nil
}

func (e *executor) SetNetworkBootstrapKeys([]*api.EncryptionKey) error {
	return nil
}

func (e *executor) Secrets() exec.SecretsManager {
	return e.secrets
}
