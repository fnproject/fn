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
		mock          models.Datastore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{datastore.NewMock(), "/v1/apps/a/routes", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{datastore.NewMock(), "/v1/apps/a/routes", `{ }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{datastore.NewMock(), "/v1/apps/a/routes", `{ "path": "/myroute" }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{datastore.NewMock(), "/v1/apps/a/routes", `{ "route": { } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingPath},
		{datastore.NewMock(), "/v1/apps/a/routes", `{ "route": { "path": "/myroute" } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingImage},
		{datastore.NewMock(), "/v1/apps/a/routes", `{ "route": { "image": "iron/hello" } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingPath},
		{datastore.NewMock(), "/v1/apps/a/routes", `{ "route": { "image": "iron/hello", "path": "myroute" } }`, http.StatusBadRequest, models.ErrRoutesValidationInvalidPath},
		{datastore.NewMock(), "/v1/apps/$/routes", `{ "route": { "image": "iron/hello", "path": "/myroute" } }`, http.StatusInternalServerError, models.ErrAppsValidationInvalidName},

		// success
		{datastore.NewMock(), "/v1/apps/a/routes", `{ "route": { "image": "iron/hello", "path": "/myroute" } }`, http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.mock, &mqs.Mock{}, rnr, tasks)

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, srv.Router, "POST", test.path, body)

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
		{datastore.NewMock(), "/v1/apps/a/routes/missing", "", http.StatusNotFound, nil},
		{datastore.NewMockInit(nil,
			[]*models.Route{
				{Path: "/myroute", AppName: "a"},
			},
		), "/v1/apps/a/routes/myroute", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, rnr, tasks)
		_, rec := routerRequest(t, srv.Router, "DELETE", test.path, nil)

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
	srv := testServer(datastore.NewMock(), &mqs.Mock{}, rnr, tasks)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/a/routes", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

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

	srv := testServer(datastore.NewMock(), &mqs.Mock{}, rnr, tasks)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/a/routes/myroute", "", http.StatusNotFound, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

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
		{datastore.NewMock(), "/v1/apps/a/routes/myroute/do", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{datastore.NewMock(), "/v1/apps/a/routes/myroute/do", `{}`, http.StatusBadRequest, models.ErrRoutesMissingNew},

		// success
		{datastore.NewMockInit(nil,
			[]*models.Route{
				{
					AppName: "a",
					Path:    "/myroute/do",
				},
			},
		), "/v1/apps/a/routes/myroute/do", `{ "route": { "image": "iron/hello" } }`, http.StatusOK, nil},

		// Addresses #381
		{datastore.NewMockInit(nil,
			[]*models.Route{
				{
					AppName: "a",
					Path:    "/myroute/do",
				},
			},
		), "/v1/apps/a/routes/myroute/do", `{ "route": { "path": "/otherpath" } }`, http.StatusBadRequest, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, rnr, tasks)

		body := bytes.NewBuffer([]byte(test.body))

		_, rec := routerRequest(t, srv.Router, "PATCH", test.path, body)

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
