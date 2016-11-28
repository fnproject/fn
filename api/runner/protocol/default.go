package protocol

import (
	"context"

	"github.com/iron-io/functions/api/runner/task"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct {
}

func (p *DefaultProtocol) IsStreamable() bool {
	return false
}

func (p *DefaultProtocol) Dispatch(ctx context.Context, t task.Request) error {
	return nil
}
