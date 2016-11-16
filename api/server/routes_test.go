package server

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/iron-io/functions/api/datastore"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/mqs"
)

func TestRouteCreate(t *testing.T) {
	buf := setLogBuffer()

	for i, test := range []struct {
		mock          *datastore.Mock
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{&datastore.Mock{}, "/v1/apps/a/routes", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{&datastore.Mock{}, "/v1/apps/a/routes", `{ }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{&datastore.Mock{}, "/v1/apps/a/routes", `{ "path": "/myroute" }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{&datastore.Mock{}, "/v1/apps/a/routes", `{ "route": { } }`, http.StatusInternalServerError, models.ErrRoutesValidationMissingPath},
		{&datastore.Mock{}, "/v1/apps/a/routes", `{ "route": { "path": "/myroute" } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingImage},
		{&datastore.Mock{}, "/v1/apps/a/routes", `{ "route": { "image": "iron/hello" } }`, http.StatusInternalServerError, models.ErrRoutesValidationMissingPath},
		{&datastore.Mock{}, "/v1/apps/a/routes", `{ "route": { "image": "iron/hello", "path": "myroute" } }`, http.StatusInternalServerError, models.ErrRoutesValidationInvalidPath},
		{&datastore.Mock{}, "/v1/apps/$/routes", `{ "route": { "image": "iron/hello", "path": "/myroute" } }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},

		// success
		{&datastore.Mock{}, "/v1/apps/a/routes", `{ "route": { "image": "iron/hello", "path": "/myroute" } }`, http.StatusCreated, nil},
	} {
		s := New(test.mock, &mqs.Mock{}, testRunner(t))
		router := testRouter(s)

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "POST", test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`, but it was `%s`",
					i, test.expectedError.Error(), resp.Error.Message)
			}
		}
	}
}

func TestRouteDelete(t *testing.T) {
	buf := setLogBuffer()
	s := New(&datastore.Mock{}, &mqs.Mock{}, testRunner(t))
	router := testRouter(s)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/a/routes", "", http.StatusTemporaryRedirect, nil},
		{"/v1/apps/a/routes/myroute", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, router, "DELETE", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestRouteList(t *testing.T) {
	buf := setLogBuffer()
	s := New(&datastore.Mock{}, &mqs.Mock{}, testRunner(t))
	router := testRouter(s)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/a/routes", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestRouteGet(t *testing.T) {
	buf := setLogBuffer()
	s := New(&datastore.Mock{}, &mqs.Mock{}, testRunner(t))
	router := testRouter(s)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/a/routes/myroute", "", http.StatusNotFound, nil},
	} {
		_, rec := routerRequest(t, router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestRouteUpdate(t *testing.T) {
	buf := setLogBuffer()
	s := New(&datastore.Mock{}, &mqs.Mock{}, testRunner(t))
	router := testRouter(s)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{"/v1/apps/a/routes/myroute/do", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{"/v1/apps/a/routes/myroute/do", `{}`, http.StatusBadRequest, models.ErrRoutesMissingNew},

		// success
		{"/v1/apps/a/routes/myroute/do", `{ "route": { "image": "iron/hello", "path": "/myroute" } }`, http.StatusOK, nil},
	} {
		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "PUT", test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}
