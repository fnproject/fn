package protocol

import (
	"context"

	"github.com/fnproject/fn/api/models"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct {
}

func (p *DefaultProtocol) IsStreamable() bool {
	return false
}

func (p *DefaultProtocol) Dispatch(context.Context, *models.Task) error {
	return nil
}
