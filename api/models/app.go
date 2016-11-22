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
	ErrAppsNotFound         = errors.New("App not found")
	ErrAppsNothingToUpdate  = errors.New("Nothing to update")
	ErrAppsRemoving         = errors.New("Could not remove app from datastore")
	ErrDeleteAppsWithRoutes = errors.New("Cannot remove apps with routes")
	ErrAppsUpdate           = errors.New("Could not update app")
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

type AppFilter struct {
}
