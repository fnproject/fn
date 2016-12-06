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
	tasks := mockTasksConduit()
	defer close(tasks)

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
		rnr, cancel := testRunner(t)
		router := testRouter(test.mock, &mqs.Mock{}, rnr, tasks)

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
		cancel()
	}
}

func TestRouteDelete(t *testing.T) {
	buf := setLogBuffer()
	tasks := mockTasksConduit()
	defer close(tasks)

	for i, test := range []struct {
		ds            models.Datastore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{&datastore.Mock{}, "/v1/apps/a/routes", "", http.StatusTemporaryRedirect, nil},
		{&datastore.Mock{}, "/v1/apps/a/routes/missing", "", http.StatusNotFound, nil},
		{&datastore.Mock{
			Routes: []*models.Route{
				{Path: "/myroute", AppName: "a"},
			},
		}, "/v1/apps/a/routes/myroute", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		router := testRouter(test.ds, &mqs.Mock{}, rnr, tasks)
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
		cancel()
	}
}

func TestRouteList(t *testing.T) {
	buf := setLogBuffer()
	tasks := mockTasksConduit()
	defer close(tasks)

	rnr, cancel := testRunner(t)
	defer cancel()
	router := testRouter(&datastore.Mock{}, &mqs.Mock{}, rnr, tasks)

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
	tasks := mockTasksConduit()
	defer close(tasks)

	rnr, cancel := testRunner(t)
	defer cancel()

	router := testRouter(&datastore.Mock{}, &mqs.Mock{}, rnr, tasks)

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
	tasks := mockTasksConduit()
	defer close(tasks)

	for i, test := range []struct {
		ds            models.Datastore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{&datastore.Mock{}, "/v1/apps/a/routes/myroute/do", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{&datastore.Mock{}, "/v1/apps/a/routes/myroute/do", `{}`, http.StatusBadRequest, models.ErrRoutesMissingNew},

		// success
		{&datastore.Mock{
			Routes: []*models.Route{
				{
					AppName: "a",
					Path:    "/myroute/do",
				},
			},
		}, "/v1/apps/a/routes/myroute/do", `{ "route": { "image": "iron/hello", "path": "/myroute" } }`, http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		router := testRouter(test.ds, &mqs.Mock{}, rnr, tasks)

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
		cancel()
	}
}
