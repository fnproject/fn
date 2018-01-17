package protocol

import (
	"io"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct{}

func (p *DefaultProtocol) IsStreamable() bool { return false }
func (d *DefaultProtocol) Dispatch(ci CallInfo, w io.Writer) error {
	return nil
}
