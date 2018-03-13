package models

import (
	"context"
	"io"
	"net/http"
	"time"
)

// RunnerPool is the abstraction for getting an ordered list of runners to try for a call
type RunnerPool interface {
	Runners(call RunnerCall) []Runner
	Shutdown()
}

// Runner is the interface to invoke the execution of a function call on a specific runner
type Runner interface {
	TryExec(ctx context.Context, call RunnerCall) (bool, error)
	Close()
	Address() string
}

// RunnerCall provides interface-based accessors to agent.call private fields
type RunnerCall interface {
	SlotDeadline() time.Time
	Request() *http.Request
	ResponseWriter() io.Writer
	StdErr() io.ReadWriteCloser
	Model() *Call
}
