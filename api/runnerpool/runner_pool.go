package runnerpool

import (
	"context"
	"github.com/fnproject/fn/api/event"
	"io"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
)

// Placer implements a placement strategy for calls that are load-balanced
// across runners in a pool
type Placer interface {
	PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) (*event.Event, error)
}

// RunnerPool is the abstraction for getting an ordered list of runners to try for a call
type RunnerPool interface {
	// returns an error for unrecoverable errors that should not be retried
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

// RunnerStatus is general information on Runner health as returned by Runner::Status() call
type RunnerStatus struct {
	ActiveRequestCount int32           // Number of active running requests on Runner
	RequestsReceived   uint64          // Number of requests received by Runner
	RequestsHandled    uint64          // Number of requests handled without NACK by Runner
	StatusFailed       bool            // True if Status execution failed
	Cached             bool            // True if Status was provided from cache
	StatusId           string          // Call ID for Status
	Details            string          // General/Debug Log information
	ErrorCode          int32           // If StatusFailed, then error code is set
	ErrorStr           string          // Error details if StatusFailed and ErrorCode is set
	CreatedAt          common.DateTime // Status creation date at Runner
	StartedAt          common.DateTime // Status execution date at Runner
	CompletedAt        common.DateTime // Status completion date at Runner
}

// Runner is the interface to invoke the execution of a function call on a specific runner
type Runner interface {
	// TryExec tries to place a call on a runner returning the outbound event from the runner if available,
	// response : is the outbound event, this will be set if error is nil
	// committed indicates if the call was committed on the runner (i.e. it may have executed) when this is false the call can  be retried without side effects
	TryExec(ctx context.Context, call RunnerCall) (response *event.Event, committed bool, error error)
	Status(ctx context.Context) (*RunnerStatus, error)
	Close(ctx context.Context) error
	Address() string
}

// RunnerCall provides access to the necessary details of request in order for it to be
// processed by a RunnerPool
// TODO is this needed any more or does agent.Call suffice
type RunnerCall interface {
	SlotHashId() string
	Extensions() map[string]string
	StdErr() io.ReadWriteCloser
	Model() *models.Call
}
