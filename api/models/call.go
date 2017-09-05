package models

import (
	strfmt "github.com/go-openapi/strfmt"
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
)

var possibleStatuses = [...]string{"delayed", "queued", "running", "success", "error", "cancelled"}

type CallLog struct {
	CallID string `json:"call_id"`
	Log    string `json:"log"`
}

// Call is a representation of a specific invocation of a route.
type Call struct {
	// Unique identifier representing a specific call.
	ID string `json:"id"`

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
	Status string `json:"status"`

	// App this call belongs to.
	AppName string `json:"app_name"`

	// Path of the route that is responsible for this call
	Path string `json:"path"`

	// Name of Docker image to use.
	Image string `json:"image"`

	// Number of seconds to wait before queueing the call for consumption for the
	// first time. Must be a positive integer. Calls with a delay start in state
	// "delayed" and transition to "running" after delay seconds.
	Delay int32 `json:"delay,omitempty"`

	// Type indicates whether a task is to be run synchronously or asynchronously.
	Type string `json:"type,omitempty"`

	// Format is the format to pass input into the function.
	Format string `json:"format,omitempty"`

	// Payload for the call. This is only used by async calls, to store their input.
	// TODO should we copy it into here too for debugging sync?
	Payload string `json:"payload,omitempty"`

	// Full request url that spawned this invocation.
	URL string `json:"url,omitempty"`

	// Method of the http request used to make this call.
	Method string `json:"method,omitempty"`

	// Priority of the call. Higher has more priority. 3 levels from 0-2. Calls
	// at same priority are processed in FIFO order.
	Priority *int32 `json:"priority"`

	// Maximum runtime in seconds.
	Timeout int32 `json:"timeout,omitempty"`

	// Hot function idle timeout in seconds before termination.
	IdleTimeout int32 `json:"idle_timeout,omitempty"`

	// Memory is the amount of RAM this call is allocated.
	Memory uint64 `json:"memory,omitempty"`

	// BaseEnv are the env vars for hot containers, not request specific.
	BaseEnv map[string]string `json:"base_env,omitempty"`

	// Env vars for the call. Comes from the ones set on the Route.
	EnvVars map[string]string `json:"env_vars,omitempty"`

	// Time when call completed, whether it was successul or failed. Always in UTC.
	CompletedAt strfmt.DateTime `json:"completed_at,omitempty"`

	// Time when call was submitted. Always in UTC.
	CreatedAt strfmt.DateTime `json:"created_at,omitempty"`

	// Time when call started execution. Always in UTC.
	StartedAt strfmt.DateTime `json:"started_at,omitempty"`
}

type CallFilter struct {
	Path    string
	AppName string
}
