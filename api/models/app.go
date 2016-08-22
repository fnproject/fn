package models

import (
	"errors"
	"fmt"
)

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
	ErrUsableImage         = errors.New("Image not found")
)

type App struct {
	Name   string `json:"name"`
	Routes Routes `json:"routes,omitempty"`
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
