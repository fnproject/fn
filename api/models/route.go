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

	MaxSyncTimeout  = 120  // 2 minutes
	MaxAsyncTimeout = 3600 // 1 hour
)

var RouteMaxMemory = uint64(8 * 1024)

type Routes []*Route

type Route struct {
	AppID       string          `json:"app_id" db:"app_id"`
	Path        string          `json:"path" db:"path"`
	Image       string          `json:"image" db:"image"`
	Memory      uint64          `json:"memory" db:"memory"`
	CPUs        MilliCPUs       `json:"cpus" db:"cpus"`
	Headers     Headers         `json:"headers,omitempty" db:"headers"`
	Type        string          `json:"type" db:"type"`
	Format      string          `json:"format" db:"format"`
	Timeout     int32           `json:"timeout" db:"timeout"`
	IdleTimeout int32           `json:"idle_timeout" db:"idle_timeout"`
	TmpFsSize   uint32          `json:"tmpfs_size" db:"tmpfs_size"`
	Config      Config          `json:"config,omitempty" db:"config"`
	Annotations Annotations     `json:"annotations,omitempty" db:"annotations"`
	CreatedAt   strfmt.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   strfmt.DateTime `json:"updated_at,omitempty" db:"updated_at"`
}

// SetDefaults sets zeroed field to defaults.
func (r *Route) SetDefaults() {
	if r.Memory == 0 {
		r.Memory = DefaultMemory
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
	if r.AppID == "" {
		return ErrRoutesMissingAppID
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

	if r.Format != FormatDefault && r.Format != FormatHTTP && r.Format != FormatJSON && r.Format != FormatCloudEvent {
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

	err = r.Annotations.Validate()
	if err != nil {
		return err
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
	eq = eq && r1.AppID == r2.AppID
	eq = eq && r1.Path == r2.Path
	eq = eq && r1.Image == r2.Image
	eq = eq && r1.Memory == r2.Memory
	eq = eq && r1.CPUs == r2.CPUs
	eq = eq && r1.Headers.Equals(r2.Headers)
	eq = eq && r1.Type == r2.Type
	eq = eq && r1.Format == r2.Format
	eq = eq && r1.Timeout == r2.Timeout
	eq = eq && r1.IdleTimeout == r2.IdleTimeout
	eq = eq && r1.TmpFsSize == r2.TmpFsSize
	eq = eq && r1.Config.Equals(r2.Config)
	eq = eq && r1.Annotations.Equals(r2.Annotations)
	// NOTE: datastore tests are not very fun to write with timestamp checks,
	// and these are not values the user may set so we kind of don't care.
	//eq = eq && time.Time(r1.CreatedAt).Equal(time.Time(r2.CreatedAt))
	//eq = eq && time.Time(r2.UpdatedAt).Equal(time.Time(r2.UpdatedAt))
	return eq
}

// Update updates fields in r with non-zero field values from new, and sets
// updated_at if any of the fields change. 0-length slice Header values, and
// empty-string Config values trigger removal of map entry.
func (r *Route) Update(patch *Route) {
	original := r.Clone()

	if patch.Image != "" {
		r.Image = patch.Image
	}
	if patch.Memory != 0 {
		r.Memory = patch.Memory
	}
	if patch.CPUs != 0 {
		r.CPUs = patch.CPUs
	}
	if patch.Type != "" {
		r.Type = patch.Type
	}
	if patch.Timeout != 0 {
		r.Timeout = patch.Timeout
	}
	if patch.IdleTimeout != 0 {
		r.IdleTimeout = patch.IdleTimeout
	}
	if patch.TmpFsSize != 0 {
		r.TmpFsSize = patch.TmpFsSize
	}
	if patch.Format != "" {
		r.Format = patch.Format
	}
	if patch.Headers != nil {
		if r.Headers == nil {
			r.Headers = Headers(make(http.Header))
		}
		for k, v := range patch.Headers {
			if len(v) == 0 {
				http.Header(r.Headers).Del(k)
			} else {
				r.Headers[k] = v
			}
		}
	}
	if patch.Config != nil {
		if r.Config == nil {
			r.Config = make(Config)
		}
		for k, v := range patch.Config {
			if v == "" {
				delete(r.Config, k)
			} else {
				r.Config[k] = v
			}
		}
	}

	r.Annotations = r.Annotations.MergeChange(patch.Annotations)

	if !r.Equals(original) {
		r.UpdatedAt = strfmt.DateTime(time.Now())
	}
}

type RouteFilter struct {
	PathPrefix string // this is prefix match TODO
	AppID      string // this is exact match (important for security)
	Image      string // this is exact match

	Cursor  string
	PerPage int
}
