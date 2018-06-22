package models

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
)

type App struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Config      Config          `json:"config,omitempty" db:"config"`
	Annotations Annotations     `json:"annotations,omitempty" db:"annotations"`
	SyslogURL   *string         `json:"syslog_url,omitempty" db:"syslog_url"`
	CreatedAt   common.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   common.DateTime `json:"updated_at,omitempty" db:"updated_at"`
}

func (a *App) SetDefaults() {
	if time.Time(a.CreatedAt).IsZero() {
		a.CreatedAt = common.DateTime(time.Now())
	}
	if time.Time(a.UpdatedAt).IsZero() {
		a.UpdatedAt = common.DateTime(time.Now())
	}
	if a.Config == nil {
		// keeps the json from being nil
		a.Config = map[string]string{}
	}
	if a.ID == "" {
		a.ID = id.New().String()
	}
}

func (a *App) Validate() error {
	if a.Name == "" {
		return ErrMissingName
	}
	if len(a.Name) > maxAppName {
		return ErrAppsTooLongName
	}
	for _, c := range a.Name {
		if !(unicode.IsLetter(c) || unicode.IsNumber(c) || c == '_' || c == '-') {
			return ErrAppsInvalidName
		}
	}
	err := a.Annotations.Validate()
	if err != nil {
		return err
	}

	if a.SyslogURL != nil && *a.SyslogURL != "" {
		sinks := strings.Split(*a.SyslogURL, ",")
		for _, s := range sinks {
			url, err := url.Parse(strings.TrimSpace(s))
			fail := err != nil
			if !fail {
				switch url.Scheme {
				case "udp", "tcp", "tls":
				default:
					fail = true
				}
			}
			if fail {
				return ErrInvalidSyslog(fmt.Sprintf(`invalid syslog url: "%v"`, s))
			}
		}
	}
	return nil
}

func (a *App) Clone() *App {
	clone := new(App)
	*clone = *a // shallow copy

	// now deep copy the map
	if a.Config != nil {
		clone.Config = make(Config, len(a.Config))
		for k, v := range a.Config {
			clone.Config[k] = v
		}
	}
	clone.ID = a.ID
	return clone
}

func (a1 *App) Equals(a2 *App) bool {
	// start off equal, check equivalence of each field.
	// the RHS of && won't eval if eq==false so config checking is lazy

	eq := true
	eq = eq && a1.ID == a2.ID
	eq = eq && a1.Name == a2.Name
	eq = eq && a1.Config.Equals(a2.Config)
	eq = eq && a1.Annotations.Equals(a2.Annotations)
	// NOTE: datastore tests are not very fun to write with timestamp checks,
	// and these are not values the user may set so we kind of don't care.
	//eq = eq && time.Time(a1.CreatedAt).Equal(time.Time(a2.CreatedAt))
	//eq = eq && time.Time(a1.UpdatedAt).Equal(time.Time(a2.UpdatedAt))
	return eq
}

// Update adds entries from patch to a.Config and a.Annotations, and removes entries with empty values.
func (a *App) Update(patch *App) {
	original := a.Clone()

	if patch.Config != nil {
		if a.Config == nil {
			a.Config = make(Config)
		}
		for k, v := range patch.Config {
			if v == "" {
				delete(a.Config, k)
			} else {
				a.Config[k] = v
			}
		}
	}

	if patch.SyslogURL != nil {
		if *patch.SyslogURL == "" {
			a.SyslogURL = nil // hides it from jason
		} else {
			a.SyslogURL = patch.SyslogURL
		}
	}

	a.Annotations = a.Annotations.MergeChange(patch.Annotations)

	if !a.Equals(original) {
		a.UpdatedAt = common.DateTime(time.Now())
	}
}

var _ APIError = ErrInvalidSyslog("")

type ErrInvalidSyslog string

func (e ErrInvalidSyslog) Code() int     { return http.StatusBadRequest }
func (e ErrInvalidSyslog) Error() string { return string(e) }

// AppFilter is the filter used for querying apps
type AppFilter struct {
	Name string
	// NameIn will filter by all names in the list (IN query)
	NameIn  []string
	PerPage int
	Cursor  string
}

type AppV2Filter struct {
	Name    string
	PerPage int
	Cursor  string
}
