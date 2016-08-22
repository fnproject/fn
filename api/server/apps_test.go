package server

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
)

func TestAppCreate(t *testing.T) {
	New(&models.Config{}, &datastore.Mock{}, testRunner(t))
	router := testRouter()

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{"/v1/apps", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{"/v1/apps", `{}`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{"/v1/apps", `{ "name": "Test" }`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{"/v1/apps", `{ "app": { "name": "" } }`, http.StatusInternalServerError, models.ErrAppsValidationMissingName},
		{"/v1/apps", `{ "app": { "name": "1234567890123456789012345678901" } }`, http.StatusInternalServerError, models.ErrAppsValidationTooLongName},
		{"/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},
		{"/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},

		// success
		{"/v1/apps", `{ "app": { "name": "teste" } }`, http.StatusCreated, nil},
	} {
		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "POST", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestAppDelete(t *testing.T) {
	New(&models.Config{}, &datastore.Mock{}, testRunner(t))
	router := testRouter()

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps", "", http.StatusNotFound, nil},
		{"/v1/apps/myapp", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, router, "DELETE", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestAppList(t *testing.T) {
	New(&models.Config{}, &datastore.Mock{}, testRunner(t))
	router := testRouter()

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestAppGet(t *testing.T) {
	New(&models.Config{}, &datastore.Mock{}, testRunner(t))
	router := testRouter()

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/myapp", "", http.StatusNotFound, nil},
	} {
		_, rec := routerRequest(t, router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestAppUpdate(t *testing.T) {
	New(&models.Config{}, &datastore.Mock{}, testRunner(t))
	router := testRouter()

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{"/v1/apps/myapp", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{"/v1/apps/myapp", `{ "name": "" }`, http.StatusInternalServerError, models.ErrAppsValidationMissingName},
		{"/v1/apps/myapp", `{ "name": "1234567890123456789012345678901" }`, http.StatusInternalServerError, models.ErrAppsValidationTooLongName},
		{"/v1/apps/myapp", `{ "name": "&&%@!#$#@$" }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},
		{"/v1/apps/myapp", `{ "name": "&&%@!#$#@$" }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},

		// success
		{"/v1/apps/myapp", `{ "name": "teste" }`, http.StatusOK, nil},
	} {
		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "PUT", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}
