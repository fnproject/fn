package models

import (
	"errors"
	"fmt"
	"net/http"
)

const (
	// MaxLengthAppName is the max length for an app name
	MaxLengthAppName = 255
	// MaxLengthFnName is the max length for an fn name
	MaxLengthFnName = 255
	// MaxLengthTriggerName is the max length for a trigger name
	MaxLengthTriggerName = 255
)

var (
	ErrMethodNotAllowed = err{
		code:  http.StatusMethodNotAllowed,
		error: errors.New("Method not allowed"),
	}

	ErrInvalidJSON = err{
		code:  http.StatusBadRequest,
		error: errors.New("Invalid JSON"),
	}
	ErrClientCancel = err{
		// The special custom error code to close connection without any response
		code:  444,
		error: errors.New("Client cancelled context"),
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

	ErrDetachUnsupported = err{
		code:  http.StatusNotImplemented,
		error: errors.New("Detach call functions are not supported on this server"),
	}

	ErrCallHandlerNotFound = err{
		code:  http.StatusInternalServerError,
		error: errors.New("Unable to find the call handle"),
	}
	ErrServiceReservationFailure = err{
		code:  http.StatusInternalServerError,
		error: errors.New("Unable to service the request for the reservation period"),
	}
	// func errors

	ErrDockerPullTimeout = ferr{
		code:  http.StatusGatewayTimeout,
		error: errors.New("Image pull timed out"),
	}
	ErrFunctionResponseTooBig = ferr{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("function response body too large"),
	}
	ErrFunctionResponseHdrTooBig = ferr{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("function response header too large"),
	}
	ErrFunctionResponse = ferr{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("error receiving function response"),
	}
	ErrFunctionFailed = ferr{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("function failed"),
	}
	ErrFunctionInvalidResponse = ferr{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("invalid function response"),
	}
	ErrFunctionPrematureWrite = ferr{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("function invoked write before receiving entire request"),
	}
	ErrFunctionWriteRequest = ferr{
		code:  http.StatusBadGateway,
		error: fmt.Errorf("function closed pipe while receiving request"),
	}
	ErrRequestContentTooBig = ferr{
		code:  http.StatusRequestEntityTooLarge,
		error: fmt.Errorf("Request content too large"),
	}
	ErrCallTimeout = ferr{
		code:  http.StatusGatewayTimeout,
		error: errors.New("Timed out"),
	}
	ErrContainerInitFail = ferr{
		code:  http.StatusBadGateway,
		error: errors.New("Container failed to initialize, please ensure you are using the latest fdk and check the logs"),
	}
	ErrContainerInitTimeout = ferr{
		code:  http.StatusGatewayTimeout,
		error: errors.New("Container initialization timed out, please ensure you are using the latest fdk and check the logs"),
	}

	ErrSyslogUnavailable = ferr{
		code:  http.StatusInternalServerError,
		error: errors.New("Syslog Unavailable"),
	}
	ErrRequestLimitExceeded = ferr{
		code:  http.StatusTooManyRequests,
		error: errors.New("Request capacity for user exceeded"),
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

var _ APIError = err{}

func (e err) Code() int { return e.code }

// NewAPIError returns an APIError given a code and error
func NewAPIError(code int, e error) APIError { return err{code, e} }

// IsAPIError returns whether err implements APIError
func IsAPIError(e error) bool {
	_, ok := e.(APIError)
	return ok
}

// GetAPIErrorCode returns 0 if an error is not an APIError, or the result
// of the Code() method from an APIError
func GetAPIErrorCode(e error) int {
	err, ok := e.(APIError)
	if ok {
		return err.Code()
	}
	return 0
}

// FuncError is an error that is the function's fault, that uses the
// APIError but distinguishes fault to function specific errors
type FuncError interface {
	APIError
	// no-op method (needed to make the interface unique)
	ImplementsFuncError()
}

type ferr struct {
	code int
	error
}

var _ FuncError = ferr{}
var _ APIError = ferr{}

func (e ferr) ImplementsFuncError() {}
func (e ferr) Code() int            { return e.code }

// NewFuncError returns a FuncError
func NewFuncError(err APIError) error { return ferr{code: err.Code(), error: err} }

// IsFuncError checks if err is of type FuncError
func IsFuncError(err error) bool { _, ok := err.(FuncError); return ok }

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
