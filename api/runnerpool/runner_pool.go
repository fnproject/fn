package runnerpool

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
)

// Placer implements a placement strategy for calls that are load-balanced
// across runners in a pool
type Placer interface {
	PlaceCall(ctx context.Context, rp RunnerPool, call RunnerCall) error
	GetPlacerConfig() PlacerConfig
}

// RunnerPool is the abstraction for getting an ordered list of runners to try for a call
type RunnerPool interface {
	// returns an error for unrecoverable errors that should not be retried
	Runners(ctx context.Context, call RunnerCall) ([]Runner, error)
	Shutdown(ctx context.Context) error
}

// RunnerStatus is general information on Runner health as returned by Runner::Status() call
type RunnerStatus struct {
	ActiveRequestCount int32           // Number of active running requests on Runner
	RequestsReceived   uint64          // Number of requests received by Runner
	RequestsHandled    uint64          // Number of requests handled without NACK by Runner
	KdumpsOnDisk       uint64          // Number of kdumps on disk
	StatusFailed       bool            // True if Status execution failed
	Cached             bool            // True if Status was provided from cache
	StatusId           string          // Call ID for Status
	Details            string          // General/Debug Log information
	ErrorCode          int32           // If StatusFailed, then error code is set
	ErrorStr           string          // Error details if StatusFailed and ErrorCode is set
	CreatedAt          common.DateTime // Status creation date at Runner
	StartedAt          common.DateTime // Status execution date at Runner
	CompletedAt        common.DateTime // Status completion date at Runner
	SchedulerDuration  time.Duration   // Amount of time runner scheduler spent on the request
	ExecutionDuration  time.Duration   // Amount of time runner spent on function execution
	IsNetworkDisabled  bool            // True if network on runner is offline
}

// Runner is the interface to invoke the execution of a function call on a specific runner
type Runner interface {
	TryExec(ctx context.Context, call RunnerCall) (bool, error)
	Status(ctx context.Context) (*RunnerStatus, error)
	Close(ctx context.Context) error
	Address() string
}

// RunnerCall provides access to the necessary details of request in order for it to be
// processed by a RunnerPool
type RunnerCall interface {
	SlotHashId() string
	Extensions() map[string]string
	RequestBody() io.ReadCloser
	ResponseWriter() http.ResponseWriter
	Model() *models.Call
	// For metrics/stats, add special accounting for time spent in customer code
	AddUserExecutionTime(dur time.Duration)
	GetUserExecutionTime() *time.Duration
}
