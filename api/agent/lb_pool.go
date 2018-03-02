package agent

import (
	"context"

	"github.com/fnproject/fn/poolmanager"
)

// NodePool provides information about pools of runners and receives capacity demands
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
	Address() string
}
