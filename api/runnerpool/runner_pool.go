package runnerpool

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/fnproject/fn/api/models"
)

// RunnerPool is the abstraction for getting an ordered list of runners to try for a call
type RunnerPool interface {
	Runners(call RunnerCall) ([]Runner, error)
	Shutdown(context.Context) error
}

// Runner is the interface to invoke the execution of a function call on a specific runner
type Runner interface {
	TryExec(ctx context.Context, call RunnerCall) (bool, error)
	Close(ctx context.Context) error
	Address() string
}

// RunnerCall provides access to the necessary details of request in order for it to be
// processed by a RunnerPool
type RunnerCall interface {
	SlotDeadline() time.Time
	Request() *http.Request
	ResponseWriter() http.ResponseWriter
	StdErr() io.ReadWriteCloser
	Model() *models.Call
}
