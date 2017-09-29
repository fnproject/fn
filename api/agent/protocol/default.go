package protocol

import (
	"io"
	"net/http"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct{}

func (p *DefaultProtocol) IsStreamable() bool                            { return false }
func (d *DefaultProtocol) Dispatch(w io.Writer, req *http.Request) error { return nil }
