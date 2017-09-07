package models

import (
	"errors"
	"fmt"
	"net/http"
)

// TODO we can put constants all in this file too
const (
	maxAppName = 30
)

var (
	ErrInvalidJSON = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid JSON"),
	}
	ErrCallTimeout = err{
		code:  http.StatusGatewayTimeout,
		error: errors.New("Timed out"),
	}
	ErrAppsValidationMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing app name"),
	}
	ErrAppsValidationTooLongName = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("App name must be %v characters or less", maxAppName),
	}
	ErrAppsValidationInvalidName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid app name"),
	}
	ErrAppsAlreadyExists = err{
		code:  http.StatusConflict,
		error: errors.New("App already exists"),
	}
	ErrAppsMissingNew = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing new application"),
	}
	ErrAppsNameImmutable = err{
		code:  http.StatusConflict,
		error: errors.New("Could not update app - name is immutable"),
	}
	ErrAppsNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("App not found"),
	}
	ErrDeleteAppsWithRoutes = err{
		code:  http.StatusConflict,
		error: errors.New("Cannot remove apps with routes"),
	}
	ErrDatastoreEmptyAppName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing app name"),
	}
	ErrDatastoreEmptyRoutePath = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route name"),
	}
	ErrDatastoreEmptyApp = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing app"),
	}
	ErrDatastoreEmptyRoute = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route"),
	}
	ErrDatastoreEmptyKey = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing key"),
	}
	ErrDatastoreEmptyCallID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing call ID"),
	}
	ErrInvalidPayload = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid payload"),
	}
	ErrRoutesAlreadyExists = err{
		code:  http.StatusConflict,
		error: errors.New("Route already exists"),
	}
	ErrRoutesMissingNew = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing new route"),
	}
	ErrRoutesNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Route not found"),
	}
	ErrRoutesPathImmutable = err{
		code:  http.StatusConflict,
		error: errors.New("Could not update route - path is immutable"),
	}
	ErrRoutesValidationFoundDynamicURL = err{
		code:  http.StatusBadRequest,
		error: errors.New("Dynamic URL is not allowed"),
	}
	ErrRoutesValidationInvalidPath = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid Path format"),
	}
	ErrRoutesValidationInvalidType = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid route Type"),
	}
	ErrRoutesValidationInvalidFormat = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid route Format"),
	}
	ErrRoutesValidationMissingAppName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route AppName"),
	}
	ErrRoutesValidationMissingImage = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Image"),
	}
	ErrRoutesValidationMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Name"),
	}
	ErrRoutesValidationMissingPath = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Path"),
	}
	ErrRoutesValidationMissingType = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Type"),
	}
	ErrRoutesValidationPathMalformed = err{
		code:  http.StatusBadRequest,
		error: errors.New("Path malformed"),
	}
	ErrRoutesValidationNegativeTimeout = err{
		code:  http.StatusBadRequest,
		error: errors.New("Negative timeout"),
	}
	ErrRoutesValidationNegativeIdleTimeout = err{
		code:  http.StatusBadRequest,
		error: errors.New("Negative idle timeout"),
	}
	ErrCallNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Call not found"),
	}
	ErrCallLogNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Call log not found"),
	}
)

// any error that implements this interface will return an API response
// with the provided status code and error message body
type APIError interface {
	Code() int
	error
}

type err struct {
	code int
	error
}

func (e err) Code() int { return e.code }

func NewAPIError(code int, e error) APIError { return err{code, e} }

// uniform error output
type Error struct {
	Error *ErrorBody `json:"error,omitempty"`
}

func (m *Error) Validate() error {
	return nil
}
