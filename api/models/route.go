package models

import (
	"errors"
	"net/http"
	"path"

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
	AppName string      `json:"appname,omitempty"`
	Path    string      `json:"path,omitempty"`
	Image   string      `json:"image,omitempty"`
	Headers http.Header `json:"headers,omitempty"`
}

var (
	ErrRoutesValidationMissingName    = errors.New("Missing route Name")
	ErrRoutesValidationMissingImage   = errors.New("Missing route Image")
	ErrRoutesValidationMissingAppName = errors.New("Missing route AppName")
	ErrRoutesValidationMissingPath    = errors.New("Missing route Path")
	ErrRoutesValidationInvalidPath    = errors.New("Invalid Path format")
)

func (r *Route) Validate() error {
	var res []error

	if r.Image == "" {
		res = append(res, ErrRoutesValidationMissingImage)
	}

	if r.AppName == "" {
		res = append(res, ErrRoutesValidationMissingAppName)
	}

	if r.Path == "" {
		res = append(res, ErrRoutesValidationMissingPath)
	}

	if !path.IsAbs(r.Path) {
		res = append(res, ErrRoutesValidationInvalidPath)
	}

	if len(res) > 0 {
		return apiErrors.CompositeValidationError(res...)
	}

	return nil
}

type RouteFilter struct {
	Path    string
	AppName string
	Image   string
}
