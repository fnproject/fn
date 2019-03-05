package server

import (
	"errors"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestErrorAndStatusCodeWithAPIError(t *testing.T) {
	l := logrus.New()
	err := models.ErrUnsupportedMediaType
	w := httptest.NewRecorder()
	actualErr, actualStatus := getErrorAndStatusCode(err, w, l)
	if actualStatus != err.Code() {
		t.Fatalf("Wrong error code, expected %d got %d", err.Code(), actualStatus)
	}
	if actualErr.Error() != err.Error() {
		t.Fatalf("Wrong error message, expected %s got %s", err.Error(), actualErr.Error())
	}
}

func TestErrorAndStatusCodeWithRetryableError(t *testing.T) {
	l := logrus.New()
	delay := 15
	err := models.NewRetryableError(models.ErrCallTimeoutServerBusy, delay, models.ErrCallTimeoutServerBusy.Code())
	w := httptest.NewRecorder()
	actualErr, actualStatus := getErrorAndStatusCode(err, w, l)

	if actualStatus != err.InnerCode() {
		t.Fatalf("Wrong error code, expected %d got %d", err.InnerCode(), actualStatus)
	}

	if actualErr.Error() != err.Error() {
		t.Fatalf("Wrong error message, expected %s got %s", err.Error(), actualErr.Error())
	}

	actualDelay := w.Header().Get("Retry-After")
	if actualDelay != strconv.Itoa(delay) {
		t.Fatalf("Wrong value set for Retry-After header, expected %s got %s", strconv.Itoa(delay), actualDelay)
	}
}

func TestErrorAndStatusCodeWithGenericError(t *testing.T) {
	expectedErr := ErrInternalServerError
	expectedStatus := http.StatusInternalServerError

	l := logrus.New()
	err := errors.New("This is a generic error")
	w := httptest.NewRecorder()
	actualErr, actualStatus := getErrorAndStatusCode(err, w, l)

	if actualStatus != expectedStatus {
		t.Fatalf("Wrong error code, expected %d got %d", expectedStatus, actualStatus)
	}

	if actualErr.Error() != expectedErr.Error() {
		t.Fatalf("Wrong error message, expected %s got %s", expectedErr, actualErr.Error())
	}

}

// TestRetryableErrorIsNotAPIError veirifies that the models.retryableError doesn't implement models.APIError
// The distinction between those 2 types is important as it is used to drive different behaviours
func TestRetryableErrorIsNotAPIError(t *testing.T) {
	err := models.NewRetryableError(models.ErrCallTimeoutServerBusy, 15, models.ErrCallTimeoutServerBusy.Code())
	if _, ok := err.(models.APIError); ok {
		t.Fatalf("NewRetryable errors return an error which implements RetryableError interface AND APIError. It must not implement the APIError interface ")
	}
}
