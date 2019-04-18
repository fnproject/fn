package models

import (
	"net/http"

	"github.com/fnproject/fn/api/agent/drivers/stats"
	"github.com/fnproject/fn/api/common"
)

const (
	// TypeNone ...
	TypeNone = ""
	// TypeSync ...
	TypeSync = "sync"
	// TypeDetached is used for calls which return an ack to the caller as soon as the call starts
	TypeDetached = "detached"
)

var possibleStatuses = [...]string{"delayed", "queued", "running", "success", "error", "cancelled"}

// Call is a representation of a specific invocation of a fn.
type Call struct {
	// Unique identifier representing a specific call.
	ID string `json:"id" db:"id"`

	// NOTE: this is stale, retries are not implemented atm, but this is nice, so leaving
	//  States and valid transitions.
	//
	//                  +---------+
	//        +---------> delayed <----------------+
	//                  +----+----+                |
	//                       |                     |
	//                       |                     |
	//                  +----v----+                |
	//        +---------> queued  <----------------+
	//                  +----+----+                *
	//                       |                     *
	//                       |               retry * creates new call
	//                  +----v----+                *
	//                  | running |                *
	//                  +--+-+-+--+                |
	//           +---------|-|-|-----+-------------+
	//       +---|---------+ | +-----|---------+   |
	//       |   |           |       |         |   |
	// +-----v---^-+      +--v-------^+     +--v---^-+
	// | success   |      | cancelled |     |  error |
	// +-----------+      +-----------+     +--------+
	//
	// * delayed - has a delay.
	// * queued - Ready to be consumed when it's turn comes.
	// * running - Currently consumed by a runner which will attempt to process it.
	// * success - (or complete? success/error is common javascript terminology)
	// * error - Something went wrong. In this case more information can be obtained
	//   by inspecting the "reason" field.
	//   - timeout
	//   - killed - forcibly killed by worker due to resource restrictions or access
	//     violations.
	//   - bad_exit - exited with non-zero status due to program termination/crash.
	// * cancelled - cancelled via API. More information in the reason field.
	//   - client_request - Request was cancelled by a client.
	Status string `json:"status" db:"status"`

	// Name of Docker image to use.
	Image string `json:"image,omitempty" db:"-"`

	// Number of seconds to wait before queueing the call for consumption for the
	// first time. Must be a positive integer. Calls with a delay start in state
	// "delayed" and transition to "running" after delay seconds.
	Delay int32 `json:"delay,omitempty" db:"-"`

	// Type indicates a call's type
	Type string `json:"type,omitempty" db:"-"`

	// Payload for the call. This is only used by async calls, to store their input.
	// TODO should we copy it into here too for debugging sync?
	Payload string `json:"payload,omitempty" db:"-"`

	// Full request url that spawned this invocation.
	URL string `json:"url,omitempty" db:"-"`

	// Method of the http request used to make this call.
	Method string `json:"method,omitempty" db:"-"`

	// Maximum runtime in seconds.
	Timeout int32 `json:"timeout,omitempty" db:"-"`

	// Hot function idle timeout in seconds before termination.
	IdleTimeout int32 `json:"idle_timeout,omitempty" db:"-"`

	// Tmpfs size in megabytes.
	TmpFsSize uint32 `json:"tmpfs_size,omitempty" db:"-"`

	// Memory is the amount of RAM this call is allocated.
	Memory uint64 `json:"memory,omitempty" db:"-"`

	// CPU as in MilliCPUs where each CPU core is split into 1000 units, specified either
	// *) milliCPUs as "100m" which is 1/10 of a CPU or
	// *) as floating point number "0.1" which is 1/10 of a CPU
	CPUs MilliCPUs `json:"cpus,omitempty" db:"-"`

	// Config is the set of configuration variables for the call
	Config Config `json:"config,omitempty" db:"-"`

	// Annotations is the set of annotations for the app/fn of the call.
	Annotations Annotations `json:"annotations,omitempty" db:"-"`

	// Headers are headers from the request that created this call
	Headers http.Header `json:"headers,omitempty" db:"-"`

	// SyslogURL is a syslog URL to send all logs to.
	SyslogURL string `json:"syslog_url,omitempty" db:"-"`

	// Time when call completed, whether it was successul or failed. Always in UTC.
	CompletedAt common.DateTime `json:"completed_at,omitempty" db:"completed_at"`

	// Time when call was submitted. Always in UTC.
	CreatedAt common.DateTime `json:"created_at,omitempty" db:"created_at"`

	// Time when call started execution. Always in UTC.
	StartedAt common.DateTime `json:"started_at,omitempty" db:"started_at"`

	// Stats is a list of metrics from this call's execution, possibly empty.
	Stats stats.Stats `json:"stats,omitempty" db:"stats"`

	// Error is the reason why the call failed, it is only non-empty if
	// status is equal to "error".
	Error string `json:"error,omitempty" db:"error"`

	// App this call belongs to.
	AppID string `json:"app_id" db:"app_id"`

	// Name of the app.
	AppName string `json:"app_name" db:"app_name"`

	// Trigger this call belongs to.
	TriggerID string `json:"trigger_id" db:"trigger_id"`

	// Fn this call belongs to.
	FnID string `json:"fn_id" db:"fn_id"`
}

type CallFilter struct {
	FnID     string //match
	FromTime common.DateTime
	ToTime   common.DateTime
	Cursor   string
	PerPage  int
}

type CallList struct {
	NextCursor string  `json:"next_cursor,omitempty"`
	Items      []*Call `json:"items"`
}
