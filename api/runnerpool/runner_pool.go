package runnerpool

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/fnproject/fn/api/models"
)

// Placer implements a placement strategy for calls that are load-balanced
// across runners in a pool
type Placer interface {
	PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error
}

// RunnerPool is the abstraction for getting an ordered list of runners to try for a call
type RunnerPool interface {
	Runners(call RunnerCall) ([]Runner, error)
	Shutdown(ctx context.Context) error
}

// PKIData encapsulates TLS certificate data
type PKIData struct {
	Ca   string
	Key  string
	Cert string
}

// MTLSRunnerFactory represents a factory method for constructing runners using mTLS
type MTLSRunnerFactory func(addr, certCommonName string, pki *PKIData) (Runner, error)

// Runner is the interface to invoke the execution of a function call on a specific runner
type Runner interface {
	TryExec(ctx context.Context, call RunnerCall) (bool, error)
	Close(ctx context.Context) error
	Address() string
}

// RunnerCall provides access to the necessary details of request in order for it to be
// processed by a RunnerPool
type RunnerCall interface {
	LbDeadline() time.Time
	RequestBody() io.ReadCloser
	ResponseWriter() http.ResponseWriter
	StdErr() io.ReadWriteCloser
	Model() *models.Call
}
