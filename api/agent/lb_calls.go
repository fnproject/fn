package agent

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
)

// These need to be in the agent package so that they can implement CallOpt (which is a function taking agent.call as its second argument)

type remoteSlot struct {
}

func (s *remoteSlot) exec(ctx context.Context, call *call) error {
	return nil
}

func (s *remoteSlot) Close(ctx context.Context) error {
	return nil
}

func (s *remoteSlot) Error() error {
	return nil
}

func WithRemoteSlot(ctx context.Context) CallOpt {
	return func(a *agent, c *call) error {
		c.reservedSlot = &remoteSlot{}
		return nil
	}
}

// RequestReader takes an agent.Call and return a ReadCloser for the request body inside it
func RequestReader(c *Call) (io.ReadCloser, error) {
	// Get the call :(((((
	cc, ok := (*c).(*call)

	if !ok {
		return nil, errors.New("Can't cast agent.Call to agent.call")
	}

	if cc.req == nil {
		return nil, errors.New("Call doesn't contain a request")
	}

	logrus.Info(cc.req)

	return cc.req.Body, nil
}

func ResponseWriter(c *Call) (*http.ResponseWriter, error) {
	cc, ok := (*c).(*call)

	if !ok {
		return nil, errors.New("Can't cast agent.Call to agent.call")
	}

	if rw, ok := cc.w.(http.ResponseWriter); ok {
		return &rw, nil
	}

	return nil, errors.New("Unable to get HTTP response writer from the call")
}
