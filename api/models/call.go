package models

import (
	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
)

const (
	// TypeNone ...
	TypeNone = ""
	// TypeSync ...
	TypeSync = "sync"
	// TypeAsync ...
	TypeAsync = "async"
)

const (
	// FormatDefault ...
	FormatDefault = "default"
	// FormatHTTP ...
	FormatHTTP = "http"
	// FormatJSON ...
	FormatJSON = "json"
	// FormatCloudEvent ...
	FormatCloudEvent = "cloudevent"
)

var possibleStatuses = [...]string{"delayed", "queued", "running", "success", "error", "cancelled"}

// Call is a representation of a specific invocation of a route.
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
	// DEPRECATED
	Status string `json:"status" db:"status"`

	// Path of the route that is responsible for this call
	// DEPRECATED
	Path string `json:"path" db:"path"`

	// Name of Docker image to use.
	Image string `json:"image,omitempty" db:"-"`

	// Number of seconds to wait before queueing the call for consumption for the
	// first time. Must be a positive integer. Calls with a delay start in state
	// "delayed" and transition to "running" after delay seconds.
	Delay int32 `json:"delay,omitempty" db:"-"`

	// Type indicates whether a task is to be run synchronously or asynchronously.
	Type string `json:"type,omitempty" db:"-"`

	// Format is the format to pass input into the function.
	Format string `json:"format,omitempty" db:"-"`

	// Payload for the call. this is  used when the event is marshalled via RPC
	// TODO should we copy it into here too for debugging sync?
	// This is intentionally  not marshalled in the JSON body
	InputEvent *event.Event

	// Priority of the call. Higher has more priority. 3 levels from 0-2. Calls
	// at same priority are processed in FIFO order.
	Priority *int32 `json:"priority,omitempty" db:"-"`

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

	// Annotations is the set of annotations for the app/route/fn of the call.
	Annotations Annotations `json:"annotations,omitempty" db:"-"`

	// SyslogURL is a syslog URL to send all logs to.
	SyslogURL string `json:"syslog_url,omitempty" db:"-"`

	// Time when call completed, whether it was successul or failed. Always in UTC.
	CompletedAt common.DateTime `json:"completed_at,omitempty" db:"completed_at"`

	// Time when call was submitted. Always in UTC.
	CreatedAt common.DateTime `json:"created_at,omitempty" db:"created_at"`

	// Time when call started execution. Always in UTC.
	StartedAt common.DateTime `json:"started_at,omitempty" db:"started_at"`

	// Stats is a list of metrics from this call's execution, possibly empty.
	Stats drivers.Stats `json:"stats,omitempty" db:"stats"`

	// Error is the reason why the call failed, it is only non-empty if
	// status is equal to "error".
	Error string `json:"error,omitempty" db:"error"`

	// App this call belongs to.
	AppID string `json:"app_id" db:"app_id"`

	// Trigger this call belongs to.
	// this is optional
	TriggerID string `json:"trigger_id,omitempty" db:"trigger_id"`

	// Fn this call belongs to.
	FnID string `json:"fn_id" db:"fn_id"`
}

type CallFilter struct {
	Path     string // match
	AppID    string // match
	FromTime common.DateTime
	ToTime   common.DateTime
	Cursor   string
	PerPage  int
}
