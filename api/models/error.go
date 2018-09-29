package models

import (
	"errors"
	"fmt"
	"net/http"
)

// TODO we can put constants all in this file too
const (
	maxAppName     = 30
	maxFnName      = 30
	MaxTriggerName = 30
)

var (
	ErrInvalidJSON = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid JSON"),
	}
	ErrClientCancel = err{
		// The special custom error code to close connection without any response
		code:  444,
		error: errors.New("Client cancelled context"),
	}
	ErrCallTimeout = err{
		code:  http.StatusGatewayTimeout,
		error: errors.New("Timed out"),
	}
	ErrCallTimeoutServerBusy = err{
		code:  http.StatusServiceUnavailable,
		error: errors.New("Timed out - server too busy"),
	}

	ErrUnsupportedMediaType = err{
		code:  http.StatusUnsupportedMediaType,
		error: errors.New("Content Type not supported")}

	ErrMissingID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing ID")}

	ErrMissingAppID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing App ID")}
	ErrMissingFnID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Fn ID")}
	ErrMissingName = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Name")}

	ErrCreatedAtProvided = err{
		code:  http.StatusBadRequest,
		error: errors.New("Trigger Created At Provided for Create")}
	ErrUpdatedAtProvided = err{
		code:  http.StatusBadRequest,
		error: errors.New("Trigger ID Provided for Create")}

	ErrDatastoreEmptyApp = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing app"),
	}
	ErrDatastoreEmptyCallID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing call ID"),
	}
	ErrDatastoreEmptyFn = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Fn"),
	}
	ErrDatastoreEmptyFnID = err{
		code:  http.StatusBadRequest,
		error: errors.New("Missing Fn ID"),
	}
	ErrInvalidPayload = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid payload"),
	}
	ErrFoundDynamicURL = err{
		code:  http.StatusBadRequest,
		error: errors.New("Dynamic URL is not allowed"),
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
	ErrInvalidMemory = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("memory value is out of range. It should be between 0 and %d", MaxMemory),
	}
	ErrCallResourceTooBig = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("Requested CPU/Memory cannot be allocated"),
	}
	ErrCallNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Call not found"),
	}
	ErrInvalidCPUs = err{
		code: http.StatusBadRequest,
		error: fmt.Errorf("Cpus is invalid. Value should be either between [%.3f and %.3f] or [%dm and %dm] milliCPU units",
			float64(MinMilliCPUs)/1000.0, float64(MaxMilliCPUs)/1000.0, MinMilliCPUs, MaxMilliCPUs),
	}
	ErrCallLogNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Call log not found"),
	}
	ErrPathNotFound = err{
		code:  http.StatusNotFound,
		error: errors.New("Path not found"),
	}
	ErrFunctionResponseTooBig = err{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("function response too large"),
	}
	ErrRequestContentTooBig = err{
		code:  http.StatusRequestEntityTooLarge,
		error: fmt.Errorf("Request content too large"),
	}
	ErrInvalidAnnotationKey = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid annotation key, annotation keys must be non-empty ascii strings excluding whitespace"),
	}
	ErrInvalidAnnotationKeyLength = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("Invalid annotation key length, annotation keys may not be larger than %d bytes", maxAnnotationKeyBytes),
	}
	ErrInvalidAnnotationValue = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid annotation value, annotation values may only be non-empty strings, numbers, objects, or arrays"),
	}
	ErrInvalidAnnotationValueLength = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("Invalid annotation value length, annotation values may not be larger than %d bytes when serialized as JSON", maxAnnotationValueBytes),
	}
	ErrTooManyAnnotationKeys = err{
		code:  http.StatusBadRequest,
		error: fmt.Errorf("Invalid annotation change, new key(s) exceed maximum permitted number of annotations keys (%d)", maxAnnotationsKeys),
	}
	ErrTooManyRequests = err{
		code:  http.StatusTooManyRequests,
		error: errors.New("Too many requests submitted"),
	}

	ErrAsyncUnsupported = err{
		code:  http.StatusBadRequest,
		error: errors.New("Async functions are not supported on this server"),
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

func IsAPIError(e error) bool {
	_, ok := e.(APIError)
	return ok
}

func GetAPIErrorCode(e error) int {
	err, ok := e.(APIError)
	if ok {
		return err.Code()
	}
	return 0
}

// ErrorWrapper uniform error output (v1)  only
type ErrorWrapper struct {
	Error *Error `json:"error,omitempty"`
}

func (m *ErrorWrapper) Validate() error {
	return nil
}

// APIErrorWrapper wraps an error with an APIError such that the APIError
// governs the HTTP response but the root error remains accessible.
type APIErrorWrapper interface {
	APIError
	RootError() error
}

type apiErrorWrapper struct {
	APIError
	root error
}

func (w apiErrorWrapper) RootError() error {
	return w.root
}

func NewAPIErrorWrapper(apiErr APIError, rootErr error) APIErrorWrapper {
	return &apiErrorWrapper{
		APIError: apiErr,
		root:     rootErr,
	}
}
