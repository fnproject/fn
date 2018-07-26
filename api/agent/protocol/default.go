package protocol

import (
	"context"
	"github.com/fnproject/fn/api/event"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct {
	source          string
	maxResponseSize uint64
}

func (p *DefaultProtocol) IsStreamable() bool { return false }
func (d *DefaultProtocol) Dispatch(ctx context.Context, input *event.Event) (*event.Event, error) {
	return nil, nil
}
