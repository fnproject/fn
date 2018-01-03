package models

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
)

const (
	DefaultTimeout     = 30  // seconds
	DefaultIdleTimeout = 30  // seconds
	DefaultMemory      = 128 // MB

	DefaultCPUQuota = 0 // unlimited

	MaxSyncTimeout  = 120  // 2 minutes
	MaxAsyncTimeout = 3600 // 1 hour
	MaxIdleTimeout  = MaxAsyncTimeout
)

var RouteMaxMemory = uint64(8 * 1024)

type Routes []*Route

type Route struct {
	AppName     string          `json:"app_name" db:"app_name"`
	Path        string          `json:"path" db:"path"`
	Image       string          `json:"image" db:"image"`
	Memory      uint64          `json:"memory" db:"memory"`
	CPUQuota    uint64          `json:"cpu_quota" db:"cpu_quota"`
	Headers     Headers         `json:"headers,omitempty" db:"headers"`
	Type        string          `json:"type" db:"type"`
	Format      string          `json:"format" db:"format"`
	Timeout     int32           `json:"timeout" db:"timeout"`
	IdleTimeout int32           `json:"idle_timeout" db:"idle_timeout"`
	Config      Config          `json:"config,omitempty" db:"config"`
	CreatedAt   strfmt.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   strfmt.DateTime `json:"updated_at,omitempty" db:"updated_at"`
}

// SetDefaults sets zeroed field to defaults.
func (r *Route) SetDefaults() {
	if r.Memory == 0 {
		r.Memory = DefaultMemory
	}

	if r.CPUQuota == 0 {
		r.CPUQuota = DefaultCPUQuota
	}

	if r.Type == TypeNone {
		r.Type = TypeSync
	}

	if r.Format == "" {
		r.Format = FormatDefault
	}

	if r.Headers == nil {
		r.Headers = Headers(http.Header{})
	}

	if r.Config == nil {
		// keeps the json from being nil
		r.Config = map[string]string{}
	}

	if r.Timeout == 0 {
		r.Timeout = DefaultTimeout
	}

	if r.IdleTimeout == 0 {
		r.IdleTimeout = DefaultIdleTimeout
	}

	if time.Time(r.CreatedAt).IsZero() {
		r.CreatedAt = strfmt.DateTime(time.Now())
	}

	if time.Time(r.UpdatedAt).IsZero() {
		r.UpdatedAt = strfmt.DateTime(time.Now())
	}
}

// Validate validates all field values, returning the first error, if any.
func (r *Route) Validate() error {
	if r.AppName == "" {
		return ErrRoutesMissingAppName
	}

	if r.Path == "" {
		return ErrRoutesMissingPath
	}

	u, err := url.Parse(r.Path)
	if err != nil {
		return ErrPathMalformed
	}

	if strings.Contains(u.Path, ":") {
		return ErrFoundDynamicURL
	}

	if !path.IsAbs(u.Path) {
		return ErrRoutesInvalidPath
	}

	if r.Image == "" {
		return ErrRoutesMissingImage
	}

	if r.Type != TypeAsync && r.Type != TypeSync {
		return ErrRoutesInvalidType
	}

	if r.Format != FormatDefault && r.Format != FormatHTTP && r.Format != FormatJSON {
		return ErrRoutesInvalidFormat
	}

	if r.Timeout <= 0 ||
		(r.Type == TypeSync && r.Timeout > MaxSyncTimeout) ||
		(r.Type == TypeAsync && r.Timeout > MaxAsyncTimeout) {
		return ErrRoutesInvalidTimeout
	}

	if r.IdleTimeout <= 0 || r.IdleTimeout > MaxIdleTimeout {
		return ErrRoutesInvalidIdleTimeout
	}

	if r.Memory < 1 || r.Memory > RouteMaxMemory {
		return ErrRoutesInvalidMemory
	}

	return nil
}

func (r *Route) Clone() *Route {
	clone := new(Route)
	*clone = *r // shallow copy

	// now deep copy the maps
	if r.Config != nil {
		clone.Config = make(Config, len(r.Config))
		for k, v := range r.Config {
			clone.Config[k] = v
		}
	}
	if r.Headers != nil {
		clone.Headers = make(Headers, len(r.Headers))
		for k, v := range r.Headers {
			// TODO technically, we need to deep copy this slice...
			clone.Headers[k] = v
		}
	}
	return clone
}

func (r1 *Route) Equals(r2 *Route) bool {
	// start off equal, check equivalence of each field.
	// the RHS of && won't eval if eq==false so config/headers checking is lazy

	eq := true
	eq = eq && r1.AppName == r2.AppName
	eq = eq && r1.Path == r2.Path
	eq = eq && r1.Image == r2.Image
	eq = eq && r1.Memory == r2.Memory
	eq = eq && r1.CPUQuota == r2.CPUQuota
	eq = eq && r1.Headers.Equals(r2.Headers)
	eq = eq && r1.Type == r2.Type
	eq = eq && r1.Format == r2.Format
	eq = eq && r1.Timeout == r2.Timeout
	eq = eq && r1.IdleTimeout == r2.IdleTimeout
	eq = eq && r1.Config.Equals(r2.Config)
	// NOTE: datastore tests are not very fun to write with timestamp checks,
	// and these are not values the user may set so we kind of don't care.
	//eq = eq && time.Time(r1.CreatedAt).Equal(time.Time(r2.CreatedAt))
	//eq = eq && time.Time(r2.UpdatedAt).Equal(time.Time(r2.UpdatedAt))
	return eq
}

// Update updates fields in r with non-zero field values from new, and sets
// updated_at if any of the fields change. 0-length slice Header values, and
// empty-string Config values trigger removal of map entry.
func (r *Route) Update(new *Route) {
	original := r.Clone()

	if new.Image != "" {
		r.Image = new.Image
	}
	if new.Memory != 0 {
		r.Memory = new.Memory
	}
	if new.CPUQuota != 0 {
		r.CPUQuota = new.CPUQuota
	}
	if new.Type != "" {
		r.Type = new.Type
	}
	if new.Timeout != 0 {
		r.Timeout = new.Timeout
	}
	if new.IdleTimeout != 0 {
		r.IdleTimeout = new.IdleTimeout
	}
	if new.Format != "" {
		r.Format = new.Format
	}
	if new.Headers != nil {
		if r.Headers == nil {
			r.Headers = Headers(make(http.Header))
		}
		for k, v := range new.Headers {
			if len(v) == 0 {
				http.Header(r.Headers).Del(k)
			} else {
				r.Headers[k] = v
			}
		}
	}
	if new.Config != nil {
		if r.Config == nil {
			r.Config = make(Config)
		}
		for k, v := range new.Config {
			if v == "" {
				delete(r.Config, k)
			} else {
				r.Config[k] = v
			}
		}
	}

	if !r.Equals(original) {
		r.UpdatedAt = strfmt.DateTime(time.Now())
	}
}

type RouteFilter struct {
	PathPrefix string // this is prefix match TODO
	AppName    string // this is exact match (important for security)
	Image      string // this is exact match

	Cursor  string
	PerPage int
}
