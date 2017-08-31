package models

import (
	"io"
	"time"

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

type FnCallLog struct {
	CallID string `json:"call_id"`
	Log    string `json:"log"`
}

// Task is a representation of a specific invocation of a route.
type Task struct {
	ID string `json:"id,omitempty"`

	/* States and valid transitions.

	                 +---------+
	       +---------> delayed <----------------+
	                 +----+----+                |
	                      |                     |
	                      |                     |
	                 +----v----+                |
	       +---------> queued  <----------------+
	                 +----+----+                *
	                      |                     *
	                      |               retry * creates new task
	                 +----v----+                *
	                 | running |                *
	                 +--+-+-+--+                |
	          +---------|-|-|-----+-------------+
	      +---|---------+ | +-----|---------+   |
	      |   |           |       |         |   |
	+-----v---^-+      +--v-------^+     +--v---^-+
	| success   |      | cancelled |     |  error |
	+-----------+      +-----------+     +--------+

	* delayed - has a delay.
	* queued - Ready to be consumed when it's turn comes.
	* running - Currently consumed by a runner which will attempt to process it.
	* success - (or complete? success/error is common javascript terminology)
	* error - Something went wrong. In this case more information can be obtained
	  by inspecting the "reason" field.
	  - timeout
	  - killed - forcibly killed by worker due to resource restrictions or access
	    violations.
	  - bad_exit - exited with non-zero status due to program termination/crash.
	* cancelled - cancelled via API. More information in the reason field.
	  - client_request - Request was cancelled by a client.


	Read Only: true
	*/
	Status string `json:"status,omitempty"`

	// App this task belongs to.
	AppName string `json:"app_name"`

	// Path of the route that is responsible for this task
	Path string `json:"path"`

	// Name of Docker image to use.
	Image string `json:"image"`

	// Number of seconds to wait before queueing the task for consumption for the first time. Must be a positive integer. Tasks with a delay start in state "delayed" and transition to "running" after delay seconds.
	Delay int32 `json:"delay,omitempty"`

	// Payload for the task. This is only used by async tasks, to store their input.
	Payload string `json:"payload,omitempty"`

	// Priority of the task. Higher has more priority. 3 levels from 0-2. Tasks at same priority are processed in FIFO order.
	Priority int32 `json:"priority"`

	// Maximum runtime in seconds.
	Timeout int32 `json:"timeout,omitempty"`

	// Hot function idle timeout in seconds before termination.
	IdleTimeout int32 `json:"idle_timeout,omitempty"`

	// Memory is the amount of RAM this task is allocated
	Memory uint64 `json:"memory,omitempty"`

	// BaseEnv are the env vars for hot containers, not request specific.
	BaseEnv map[string]string `json:"base_env,omitempty"`

	// Env vars for the task. Comes from the ones set on the Route.
	EnvVars map[string]string `json:"env_vars,omitempty"`

	// Format is the format to pass input into the function.
	// TODO plumb this in async land
	Format string `json:"format,omitempty"`

	// Time when task completed, whether it was successul or failed. Always in UTC.
	CompletedAt strfmt.DateTime `json:"completed_at,omitempty"`

	// Time when task was submitted. Always in UTC.
	CreatedAt strfmt.DateTime `json:"created_at,omitempty"`

	// Time when task started execution. Always in UTC.
	StartedAt strfmt.DateTime `json:"started_at,omitempty"`

	// Below are from Config
	ReceivedTime time.Time
	// Ready is used to await the first pull
	Ready  chan struct{}  `json:"-"`
	Stdin  io.Reader      `json:"-"`
	Stdout io.Writer      `json:"-"`
	Stderr io.WriteCloser `json:"-"` // closer for flushy poo
}

// TimeoutDuration returns Timeout as a time.Duration
func (t *Task) TimeoutDuration() time.Duration {
	return time.Duration(t.IdleTimeout) * time.Second
}

// IdleTimeoutDuration returns IdleTimeout as a time.Duration
func (t *Task) IdleTimeoutDuration() time.Duration {
	return time.Duration(t.IdleTimeout) * time.Second
}

type CallFilter struct {
	Path    string
	AppName string
}
