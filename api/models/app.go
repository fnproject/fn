// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
