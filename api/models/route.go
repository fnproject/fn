package models

import (
	"errors"
	"net/http"

	apiErrors "github.com/go-openapi/errors"
)

var (
	ErrRoutesCreate     = errors.New("Could not create route")
	ErrRoutesUpdate     = errors.New("Could not update route")
	ErrRoutesRemoving   = errors.New("Could not remove route from datastore")
	ErrRoutesGet        = errors.New("Could not get route from datastore")
	ErrRoutesList       = errors.New("Could not list routes from datastore")
	ErrRoutesNotFound   = errors.New("Route not found")
	ErrRoutesMissingNew = errors.New("Missing new route")
)

type Routes []*Route

type Route struct {
	Name    string      `json:"name"`
	AppName string      `json:"appname"`
	Path    string      `json:"path"`
	Image   string      `json:"image"`
	Headers http.Header `json:"headers,omitempty"`
}

var (
	ErrRoutesValidationName    = errors.New("Missing route Name")
	ErrRoutesValidationImage   = errors.New("Missing route Image")
	ErrRoutesValidationAppName = errors.New("Missing route AppName")
	ErrRoutesValidationPath    = errors.New("Missing route Path")
)

func (r *Route) Validate() error {
	var res []error

	if r.Name == "" {
		res = append(res, ErrRoutesValidationAppName)
	}

	if r.Image == "" {
		res = append(res, ErrRoutesValidationImage)
	}

	if r.AppName == "" {
		res = append(res, ErrRoutesValidationAppName)
	}

	if r.Path == "" {
		res = append(res, ErrRoutesValidationPath)
	}

	if len(res) > 0 {
		return apiErrors.CompositeValidationError(res...)
	}

	return nil
}

type RouteFilter struct {
	Path    string
	AppName string
}
