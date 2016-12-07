package models

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	apiErrors "github.com/go-openapi/errors"
)

const (
	defaultRouteTimeout = 30 // seconds
)

var (
	ErrInvalidPayload      = errors.New("Invalid payload")
	ErrRoutesAlreadyExists = errors.New("Route already exists")
	ErrRoutesCreate        = errors.New("Could not create route")
	ErrRoutesGet           = errors.New("Could not get route from datastore")
	ErrRoutesList          = errors.New("Could not list routes from datastore")
	ErrRoutesMissingNew    = errors.New("Missing new route")
	ErrRoutesNotFound      = errors.New("Route not found")
	ErrRoutesPathImmutable = errors.New("Could not update route - path is immutable")
	ErrRoutesRemoving      = errors.New("Could not remove route from datastore")
	ErrRoutesUpdate        = errors.New("Could not update route")
)

type Routes []*Route

type Route struct {
	AppName        string      `json:"app_name,omitempty"`
	Path           string      `json:"path,omitempty"`
	Image          string      `json:"image,omitempty"`
	Memory         uint64      `json:"memory,omitempty"`
	Headers        http.Header `json:"headers,omitempty"`
	Type           string      `json:"type,omitempty"`
	Format         string      `json:"format,omitempty"`
	MaxConcurrency int         `json:"max_concurrency,omitempty"`
	Timeout        int32       `json:"timeout,omitempty"`
	Config         `json:"config"`
}

var (
	ErrRoutesValidationFoundDynamicURL = errors.New("Dynamic URL is not allowed")
	ErrRoutesValidationInvalidPath     = errors.New("Invalid Path format")
	ErrRoutesValidationInvalidType     = errors.New("Invalid route Type")
	ErrRoutesValidationInvalidFormat   = errors.New("Invalid route Format")
	ErrRoutesValidationMissingAppName  = errors.New("Missing route AppName")
	ErrRoutesValidationMissingImage    = errors.New("Missing route Image")
	ErrRoutesValidationMissingName     = errors.New("Missing route Name")
	ErrRoutesValidationMissingPath     = errors.New("Missing route Path")
	ErrRoutesValidationMissingType     = errors.New("Missing route Type")
	ErrRoutesValidationPathMalformed   = errors.New("Path malformed")
	ErrRoutesValidationNegativeTimeout = errors.New("Negative timeout")
)

func (r *Route) Validate() error {
	var res []error

	if r.Memory == 0 {
		r.Memory = 128
	}

	if r.AppName == "" {
		res = append(res, ErrRoutesValidationMissingAppName)
	}

	if r.Path == "" {
		res = append(res, ErrRoutesValidationMissingPath)
	}

	u, err := url.Parse(r.Path)
	if err != nil {
		res = append(res, ErrRoutesValidationPathMalformed)
	}

	if strings.Contains(u.Path, ":") {
		res = append(res, ErrRoutesValidationFoundDynamicURL)
	}

	if !path.IsAbs(u.Path) {
		res = append(res, ErrRoutesValidationInvalidPath)
	}

	if r.Type == TypeNone {
		r.Type = TypeSync
	}

	if r.Type != TypeAsync && r.Type != TypeSync {
		res = append(res, ErrRoutesValidationInvalidType)
	}

	if r.Format != FormatDefault && r.Format != FormatHTTP {
		res = append(res, ErrRoutesValidationInvalidFormat)
	}

	if r.MaxConcurrency == 0 && r.Format == FormatHTTP {
		r.MaxConcurrency = 1
	}

	if r.Timeout == 0 {
		r.Timeout = defaultRouteTimeout
	} else if r.Timeout < 0 {
		res = append(res, ErrRoutesValidationNegativeTimeout)
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
