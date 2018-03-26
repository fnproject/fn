package protocol

import (
	"context"
	"io"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct{}

func (p *DefaultProtocol) IsStreamable() bool { return false }
func (d *DefaultProtocol) Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error {
	return nil
}
