package protocol

import (
	"context"
	"errors"
	"io"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner/task"
)

var errInvalidProtocol = errors.New("Invalid Protocol")

// ContainerIO defines the interface used to talk to a hot container.
// Internally, a protocol must know when to alternate between stdin and stdout.
// It returns any protocol error, if present.
type ContainerIO interface {
	IsStreamable() bool
	Dispatch(ctx context.Context, t task.Request) error
}

// Protocol defines all protocols that operates a ContainerIO.
type Protocol string

// Hot container protocols
const (
	Default Protocol = models.FormatDefault
	HTTP    Protocol = models.FormatHTTP
)

// New creates a valid protocol handler from a I/O pipe representing containers
// stdin/stdout.
func New(p Protocol, in io.Writer, out io.Reader) (ContainerIO, error) {
	switch p {
	case HTTP:
		return &HTTPProtocol{in, out}, nil
	case Default:
		return &DefaultProtocol{}, nil
	default:
		return nil, errInvalidProtocol
	}
}

// IsStreamable says whether the given protocol can be used for streaming into
// hot containers.
func IsStreamable(p string) (bool, error) {
	proto, err := New(Protocol(p), nil, nil)
	if err != nil {
		return false, err
	}
	return proto.IsStreamable(), nil
}
