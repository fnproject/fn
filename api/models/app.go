package models

import "errors"

type Apps []*App

var (
	ErrAppsCreate          = errors.New("Could not create app")
	ErrAppsUpdate          = errors.New("Could not update app")
	ErrAppsRemoving        = errors.New("Could not remove app from datastore")
	ErrAppsGet             = errors.New("Could not get app from datastore")
	ErrAppsList            = errors.New("Could not list apps from datastore")
	ErrAppsNotFound        = errors.New("App not found")
	ErrAppsNothingToUpdate = errors.New("Nothing to update")
	ErrAppsMissingNew      = errors.New("Missing new application")
)

type App struct {
	Name   string `json:"name"`
	Routes Routes `json:"routes,omitempty"`
}

var (
	ErrAppsValidationName = errors.New("Missing app name")
)

func (a *App) Validate() error {
	if a.Name == "" {
		return ErrAppsValidationName
	}
	return nil
}

type AppFilter struct {
}
