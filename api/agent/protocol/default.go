package protocol

import (
	"io"
	"net/http"

	"github.com/fnproject/fn/api/models"
)

// DefaultProtocol is the protocol used by cold-containers
type DefaultProtocol struct{}

func (p *DefaultProtocol) IsStreamable() bool { return false }
func (d *DefaultProtocol) Dispatch(call *models.Call, w io.Writer, req *http.Request) error {
	return nil
}
