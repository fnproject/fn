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
	ErrCallTimeoutServerBusy = err{
		code:  http.StatusServiceUnavailable,
		error: errors.New("Timed out - server too busy"),
	}
	ErrAppsMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing app name"),
	}
	ErrAppsTooLongName = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("App name must be %v characters or less", maxAppName),
	}
	ErrAppsInvalidName = err{
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
		error: errors.New("Could not update - name is immutable"),
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
		error: errors.New("Could not update - path is immutable"),
	}
	ErrFoundDynamicURL = err{
		code:  http.StatusBadRequest,
		error: errors.New("Dynamic URL is not allowed"),
	}
	ErrRoutesInvalidPath = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid route path format"),
	}
	ErrRoutesInvalidType = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid route Type"),
	}
	ErrRoutesInvalidFormat = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid route Format"),
	}
	ErrRoutesMissingAppName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route AppName"),
	}
	ErrRoutesMissingImage = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Image"),
	}
	ErrRoutesMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Name"),
	}
	ErrRoutesMissingPath = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Path"),
	}
	ErrRoutesMissingType = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing route Type"),
	}
	ErrPathMalformed = err{
		code:  http.StatusBadRequest,
		error: errors.New("Path malformed"),
	}
	ErrInvalidToTime = err{
		code:  http.StatusBadRequest,
		error: errors.New("to_time is not an epoch time"),
	}
	ErrInvalidFromTime = err{
		code:  http.StatusBadRequest,
		error: errors.New("from_time is not an epoch time"),
	}
	ErrRoutesInvalidTimeout = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("timeout value is too large or small. 0 < timeout < max. async max: %d sync max: %d", MaxAsyncTimeout, MaxSyncTimeout),
	}
	ErrRoutesInvalidIdleTimeout = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("idle_timeout value is too large or small. 0 < timeout < %d", MaxIdleTimeout),
	}
	ErrRoutesInvalidMemory = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("memory value is invalid. 0 < memory < %d", RouteMaxMemory),
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

// APIError any error that implements this interface will return an API response
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

// Error uniform error output
type Error struct {
	Error *ErrorBody `json:"error,omitempty"`
}

func (m *Error) Validate() error {
	return nil
}
