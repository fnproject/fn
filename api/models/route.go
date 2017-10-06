package models

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/go-openapi/strfmt"
)

const (
	DefaultTimeout     = 30  // seconds
	DefaultIdleTimeout = 30  // seconds
	DefaultMemory      = 128 // MB

	MaxSyncTimeout  = 120  // 2 minutes
	MaxAsyncTimeout = 3600 // 1 hour
	MaxIdleTimeout  = MaxAsyncTimeout
	MaxMemory       = 1024 * 8 // 8GB TODO should probably be a var of machine max?
)

type Routes []*Route

type Route struct {
	AppName     string          `json:"app_name" db:"app_name"`
	Path        string          `json:"path" db:"path"`
	Image       string          `json:"image" db:"image"`
	Memory      uint64          `json:"memory" db:"memory"`
	Headers     Headers         `json:"headers,omitempty" db:"headers"`
	Type        string          `json:"type" db:"type"`
	Format      string          `json:"format" db:"format"`
	Timeout     int32           `json:"timeout" db:"timeout"`
	IdleTimeout int32           `json:"idle_timeout" db:"idle_timeout"`
	Config      Config          `json:"config,omitempty" db:"config"`
	CreatedAt   strfmt.DateTime `json:"created_at,omitempty" db:"created_at"`
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
		r.Config = map[string]string{}
	}

	if r.Timeout == 0 {
		r.Timeout = DefaultTimeout
	}

	if r.IdleTimeout == 0 {
		r.IdleTimeout = DefaultIdleTimeout
	}
}

// Validate validates all field values, returning the first error, if any.
func (r *Route) Validate() error {
	if r.AppName == "" {
		return ErrRoutesMissingAppName
	}

	if r.Path == "" {
		return ErrRoutesMissingPath
	} else {
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

	if r.Memory < 1 || r.Memory > MaxMemory {
		return ErrRoutesInvalidMemory
	}

	return nil
}

func (r *Route) Clone() *Route {
	var clone Route
	clone.AppName = r.AppName
	clone.Path = r.Path
	clone.Update(r)
	return &clone
}

// Update updates fields in r with non-zero field values from new.
// 0-length slice Header values, and empty-string Config values trigger removal of map entry.
func (r *Route) Update(new *Route) {
	if new.Image != "" {
		r.Image = new.Image
	}
	if new.Memory != 0 {
		r.Memory = new.Memory
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
}

type RouteFilter struct {
	PathPrefix string // this is prefix match TODO
	AppName    string // this is exact match (important for security)
	Image      string // this is exact match

	Cursor  string
	PerPage int
}
