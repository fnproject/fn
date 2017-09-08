package models

import (
	"net/http"
	"net/url"
	"path"
	"strings"
)

const (
	DefaultRouteTimeout = 30 // seconds
	DefaultIdleTimeout  = 30 // seconds
)

type Routes []*Route

type Route struct {
	AppName     string      `json:"app_name"`
	Path        string      `json:"path"`
	Image       string      `json:"image"`
	Memory      uint64      `json:"memory"`
	Headers     http.Header `json:"headers"`
	Type        string      `json:"type"`
	Format      string      `json:"format"`
	Timeout     int32       `json:"timeout"`
	IdleTimeout int32       `json:"idle_timeout"`
	Config      `json:"config"`
}

// SetDefaults sets zeroed field to defaults.
func (r *Route) SetDefaults() {
	if r.Memory == 0 {
		r.Memory = 128
	}

	if r.Type == TypeNone {
		r.Type = TypeSync
	}

	if r.Format == "" {
		r.Format = FormatDefault
	}

	if r.Headers == nil {
		r.Headers = http.Header{}
	}

	if r.Config == nil {
		r.Config = map[string]string{}
	}

	if r.Timeout == 0 {
		r.Timeout = DefaultRouteTimeout
	}

	if r.IdleTimeout == 0 {
		r.IdleTimeout = DefaultIdleTimeout
	}
}

// Validate validates field values, skipping zeroed fields if skipZero is true.
// it returns the first error, if any.
func (r *Route) Validate(skipZero bool) error {
	if !skipZero {
		if r.AppName == "" {
			return ErrMissingAppName
		}

		if r.Path == "" {
			return ErrMissingPath
		}

		if r.Image == "" {
			return ErrMissingImage
		}
	}

	if !skipZero || r.Path != "" {
		u, err := url.Parse(r.Path)
		if err != nil {
			return ErrPathMalformed
		}

		if strings.Contains(u.Path, ":") {
			return ErrFoundDynamicURL
		}

		if !path.IsAbs(u.Path) {
			return ErrInvalidPath
		}
	}

	if !skipZero || r.Type != "" {
		if r.Type != TypeAsync && r.Type != TypeSync {
			return ErrInvalidType
		}
	}

	if !skipZero || r.Format != "" {
		if r.Format != FormatDefault && r.Format != FormatHTTP {
			return ErrInvalidFormat
		}
	}

	if r.Timeout < 0 {
		return ErrNegativeTimeout
	}

	if r.IdleTimeout < 0 {
		return ErrNegativeIdleTimeout
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
			r.Headers = make(http.Header)
		}
		for k, v := range new.Headers {
			if len(v) == 0 {
				r.Headers.Del(k)
			} else {
				for _, val := range v {
					r.Headers.Add(k, val)
				}
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
