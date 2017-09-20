package models

import (
	"net/http"
	"strings"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/go-openapi/strfmt"
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
)

var possibleStatuses = [...]string{"delayed", "queued", "running", "success", "error", "cancelled"}

type CallLog struct {
	CallID  string `json:"call_id" db:"id"`
	Log     string `json:"log" db:"log"`
	AppName string `json:"app_name" db:"app_name"`
}

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
	Status string `json:"status" db:"status"`

	// App this call belongs to.
	AppName string `json:"app_name" db:"app_name"`

	// Path of the route that is responsible for this call
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

	// Payload for the call. This is only used by async calls, to store their input.
	// TODO should we copy it into here too for debugging sync?
	Payload string `json:"payload,omitempty" db:"-"`

	// Full request url that spawned this invocation.
	URL string `json:"url,omitempty" db:"-"`

	// Method of the http request used to make this call.
	Method string `json:"method,omitempty" db:"-"`

	// Priority of the call. Higher has more priority. 3 levels from 0-2. Calls
	// at same priority are processed in FIFO order.
	Priority *int32 `json:"priority,omitempty" db:"-"`

	// Maximum runtime in seconds.
	Timeout int32 `json:"timeout,omitempty" db:"-"`

	// Hot function idle timeout in seconds before termination.
	IdleTimeout int32 `json:"idle_timeout,omitempty" db:"-"`

	// Memory is the amount of RAM this call is allocated.
	Memory uint64 `json:"memory,omitempty" db:"-"`

	// Env are the env vars / headers for the given call.
	Env *CallEnv `json:"env,omitempty" db:"-"`

	// Time when call completed, whether it was successul or failed. Always in UTC.
	CompletedAt strfmt.DateTime `json:"completed_at,omitempty" db:"completed_at"`

	// Time when call was submitted. Always in UTC.
	CreatedAt strfmt.DateTime `json:"created_at,omitempty" db:"created_at"`

	// Time when call started execution. Always in UTC.
	StartedAt strfmt.DateTime `json:"started_at,omitempty" db:"started_at"`

	// Stats is a list of metrics from this call's execution, possibly empty.
	Stats drivers.Stats `json:"stats,omitempty" db:"stats"`
}

// CallEnv are similar to http.Header, but since they need to be translated
// into an env var map there is extra sugar for accessing them. CallEnv also
// preserve the casing of keys when storing them, unlike http.Header. To access
// a version with the headers in http format, use HTTP().
type CallEnv struct {
	// Full list of headers
	Header map[string][]string `json:"header,omitempty"`

	// List of headers to return from B()
	B []string `json:"base,omitempty"`
}

func (r *CallEnv) Set(key, value string) {
	// delete in http form first, for casing!
	http.Header(r.Header).Del(key)

	s := [1]string{value}
	r.Header[key] = s[:]
}

func (r *CallEnv) SetBase(key, value string) {
	r.Set(key, value)
	r.B = append(r.B, key)
}

func (r *CallEnv) Base() map[string]string {
	m := make(map[string]string, len(r.B))
	for _, k := range r.B {
		m[k] = strings.Join(r.Header[k], ", ")
	}
	return m
}

func (r *CallEnv) Full() map[string]string {
	m := make(map[string]string, len(r.Header))
	for k, v := range r.Header {
		m[k] = strings.Join(v, ", ")
	}
	return m
}

func (r *CallEnv) HTTP() http.Header {
	m := make(http.Header, len(r.Header))
	for k, vs := range r.Header {
		for _, v := range vs {
			// important, for casing
			m.Add(k, v)
		}
	}
	return m
}

type CallFilter struct {
	Path     string // match
	AppName  string // match
	FromTime strfmt.DateTime
	ToTime   strfmt.DateTime
	Cursor   string
	PerPage  int
}
