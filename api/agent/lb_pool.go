package agent

import (
	"context"

	"github.com/fnproject/fn/poolmanager"
)

// NodePool is the interface to interact with Node pool manager
type NodePool interface {
	Runners(lbgID string) []Runner
	AssignCapacity(r *poolmanager.CapacityRequest)
	ReleaseCapacity(r *poolmanager.CapacityRequest)
	Shutdown()
}

// Runner is the interface to invoke the execution of a function call on a specific runner
type Runner interface {
	TryExec(ctx context.Context, call Call) (bool, error)
	Close()
}

// RunnerFactory is a factory func that creates a Runner usable by the pool.
type RunnerFactory func(addr string, lbgId string, cert string, key string, ca string) (Runner, error)

type nullRunner struct{}

func (n *nullRunner) TryExec(ctx context.Context, call Call) (bool, error) {
	return false, nil
}

func (n *nullRunner) Close() {}

var NullRunner Runner = &nullRunner{}
