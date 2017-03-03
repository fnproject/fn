package models

import (
	"errors"
	"fmt"
)

type Apps []*App

var (
	ErrAppsAlreadyExists    = errors.New("App already exists")
	ErrAppsCreate           = errors.New("Could not create app")
	ErrAppsGet              = errors.New("Could not get app from datastore")
	ErrAppsList             = errors.New("Could not list apps from datastore")
	ErrAppsMissingNew       = errors.New("Missing new application")
	ErrAppsNameImmutable    = errors.New("Could not update app - name is immutable")
	ErrAppsNotFound         = errors.New("App not found")
	ErrAppsNothingToUpdate  = errors.New("Nothing to update")
	ErrAppsRemoving         = errors.New("Could not remove app from datastore")
	ErrAppsUpdate           = errors.New("Could not update app")
	ErrDeleteAppsWithRoutes = errors.New("Cannot remove apps with routes")
	ErrUsableImage          = errors.New("Image not found")
)

type App struct {
	Name   string `json:"name"`
	Routes Routes `json:"routes,omitempty"`
	Config `json:"config"`
}

const (
	maxAppName = 30
)

var (
	ErrAppsValidationMissingName = errors.New("Missing app name")
	ErrAppsValidationTooLongName = fmt.Errorf("App name must be %v characters or less", maxAppName)
	ErrAppsValidationInvalidName = errors.New("Invalid app name")
)

func (a *App) Validate() error {
	if a.Name == "" {
		return ErrAppsValidationMissingName
	}
	if len(a.Name) > maxAppName {
		return ErrAppsValidationTooLongName
	}
	for _, c := range a.Name {
		if (c < '0' || '9' < c) && (c < 'A' || 'Z' > c) && (c < 'a' || 'z' < c) && c != '_' && c != '-' {
			return ErrAppsValidationInvalidName
		}
	}
	return nil
}

func (a *App) Clone() *App {
	var c App
	c.Name = a.Name
	if a.Routes != nil {
		for i := range a.Routes {
			c.Routes = append(c.Routes, a.Routes[i].Clone())
		}
	}
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
	// An SQL LIKE query. Empty does not filter.
	Name string
}
