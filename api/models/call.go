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

	// List of headers to return from Base()
	B []string `json:"base,omitempty"`

	// List of headers from request
	H []string `json:"req_headers,omitempty"`
}

func EnvFromReq(req *http.Request) *CallEnv {
	var env CallEnv
	// we can base our new ones off our old ones, for speed
	env.Header = map[string][]string(req.Header)

	// add these all to req headers so we can rewrite them if this thing is cold
	env.H = make([]string, 0, len(env.Header))
	for k := range env.Header {
		env.H = append(env.H, k)
	}

	return &env
}

func (r *CallEnv) Set(key, value string) {
	// if we found this it means the user is attempting to shove our 'base vars' into
	// the headers, so we can put an end to this by removing from H set
	exk := http.Header(r.Header).Get(key)
	if exk != "" {
		for i, h := range r.H {
			if strings.ToUpper(h) == strings.ToUpper(key) { // mask http header casing
				r.H = append(r.H[:i], r.H[i+1:]...)
				break
			}
		}
	}

	// delete in http form first, for casing!
	http.Header(r.Header).Del(key)

	s := [1]string{value}
	r.Header[key] = s[:]
}

func (r *CallEnv) SetBase(key, value string) {
	r.Set(key, value)
	r.B = append(r.B, key)
}

func (r *CallEnv) AddBase(key, value string) {
	r.Header[key] = append(r.Header[key], value)
	r.B = append(r.B, key)
}

func (r *CallEnv) Base() map[string]string {
	m := make(map[string]string, len(r.B))
	for _, k := range r.B {
		m[k] = strings.Join(r.Header[k], ", ")
	}
	return m
}

// full returns the entire set of env vars, and rewrites them to CGI format and
// prepends FN_HEADER to headers.
func (r *CallEnv) Full() map[string]string {
	m := make(map[string]string, len(r.Header))
	for _, h := range r.H {
		m[toEnv("FN_HEADER_"+h)] = strings.Join(r.Header[h], ", ")
	}

	for k, v := range r.Header {
		if _, ok := m[toEnv("FN_HEADER_"+k)]; ok { // blech, inefficient fail :(
			continue // we added headers already and they are special
		}
		m[k] = strings.Join(v, ", ")
	}
	return m
}

func toEnv(key string) string {
	return strings.ToUpper(strings.Replace(key, "-", "_", -1))
}

// HTTP returns the disjoint set of headers in full and base (full - base)
func (r *CallEnv) HTTP() http.Header {
	m := make(http.Header, len(r.Header))

	for k, vs := range r.Header {
		for _, v := range vs {
			// important, for casing
			m.Add(k, v)
		}
	}

	// now toss out any base keys
	for _, bk := range r.B {
		m.Del(bk)
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
