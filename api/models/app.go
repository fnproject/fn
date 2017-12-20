package models

import (
	"time"

	"github.com/go-openapi/strfmt"
)

type App struct {
	Name      string          `json:"name" db:"name"`
	Config    Config          `json:"config,omitempty" db:"config"`
	CreatedAt strfmt.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt strfmt.DateTime `json:"updated_at,omitempty" db:"updated_at"`
}

func (a *App) SetDefaults() {
	if time.Time(a.CreatedAt).IsZero() {
		a.CreatedAt = strfmt.DateTime(time.Now())
	}
	if time.Time(a.UpdatedAt).IsZero() {
		a.UpdatedAt = strfmt.DateTime(time.Now())
	}
	if a.Config == nil {
		// keeps the json from being nil
		a.Config = map[string]string{}
	}
}

func (a *App) Validate() error {
	if a.Name == "" {
		return ErrAppsMissingName
	}
	if len(a.Name) > maxAppName {
		return ErrAppsTooLongName
	}
	for _, c := range a.Name {
		if (c < '0' || '9' < c) && (c < 'A' || 'Z' > c) && (c < 'a' || 'z' < c) && c != '_' && c != '-' {
			return ErrAppsInvalidName
		}
	}
	return nil
}

func (a *App) Clone() *App {
	clone := new(App)
	*clone = *a // shallow copy

	// now deep copy the map
	if a.Config != nil {
		clone.Config = make(Config)
		for k, v := range a.Config {
			clone.Config[k] = v
		}
	}
	return clone
}

// UpdateConfig adds entries from patch to a.Config, and removes entries with empty values.
func (a *App) UpdateConfig(src *App) {
	if src.Config != nil {
		a.UpdatedAt = strfmt.DateTime(time.Now())
		if a.Config == nil {
			a.Config = make(Config)
		}
		for k, v := range src.Config {
			if v == "" {
				delete(a.Config, k)
			} else {
				a.Config[k] = v
			}
		}
	}
}

// AppFilter is the filter used for querying apps
type AppFilter struct {
	Name string
	// NameIn will filter by all names in the list (IN query)
	NameIn  []string
	PerPage int
	Cursor  string
}
