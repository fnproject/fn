package models

type App struct {
	Name   string `json:"name" db:"name"`
	Config Config `json:"config" db:"config"`
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
	var c App
	c.Name = a.Name
	if a.Config != nil {
		c.Config = make(Config)
		for k, v := range a.Config {
			c.Config[k] = v
		}
	}
	return &c
}

// UpdateConfig adds entries from patch to a.Config, and removes entries with empty values.
func (a *App) UpdateConfig(patch Config) {
	if patch != nil {
		if a.Config == nil {
			a.Config = make(Config)
		}
		for k, v := range patch {
			if v == "" {
				delete(a.Config, k)
			} else {
				a.Config[k] = v
			}
		}
	}
}

type AppFilter struct {
	Name    string // prefix query TODO implemented
	PerPage int
	Cursor  string
}
