package models

import (
	"time"
	"unicode"

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
		if !(unicode.IsLetter(c) || unicode.IsNumber(c) || c == '_' || c == '-') {
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
		clone.Config = make(Config, len(a.Config))
		for k, v := range a.Config {
			clone.Config[k] = v
		}
	}
	return clone
}

func (a1 *App) Equals(a2 *App) bool {
	// start off equal, check equivalence of each field.
	// the RHS of && won't eval if eq==false so config checking is lazy

	eq := true
	eq = eq && a1.Name == a2.Name
	eq = eq && a1.Config.Equals(a2.Config)
	// NOTE: datastore tests are not very fun to write with timestamp checks,
	// and these are not values the user may set so we kind of don't care.
	//eq = eq && time.Time(a1.CreatedAt).Equal(time.Time(a2.CreatedAt))
	//eq = eq && time.Time(a1.UpdatedAt).Equal(time.Time(a2.UpdatedAt))
	return eq
}

// Update adds entries from patch to a.Config, and removes entries with empty values.
func (a *App) Update(src *App) {
	original := a.Clone()

	if src.Config != nil {
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

	if !a.Equals(original) {
		a.UpdatedAt = strfmt.DateTime(time.Now())
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
