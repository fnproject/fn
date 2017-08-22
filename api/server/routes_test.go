package server

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

type routeTestCase struct {
	ds            models.Datastore
	logDB         models.FnLog
	method        string
	path          string
	body          string
	expectedCode  int
	expectedError error
}

func (test *routeTestCase) run(t *testing.T, i int, buf *bytes.Buffer) {
	rnr, cancel := testRunner(t)
	srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, DefaultEnqueue)

	body := bytes.NewBuffer([]byte(test.body))
	_, rec := routerRequest(t, srv.Router, test.method, test.path, body)

	if rec.Code != test.expectedCode {
		t.Log(buf.String())
		t.Errorf("Test %d: Expected status code to be %d but was %d",
			i, test.expectedCode, rec.Code)
	}

	if test.expectedError != nil {
		resp := getErrorResponse(t, rec)
		if resp.Error == nil {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected error message to have `%s`, but it was nil",
				i, test.expectedError)
		} else if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected error message to have `%s`, but it was `%s`",
				i, test.expectedError, resp.Error.Message)
		}
	}
	cancel()
	buf.Reset()
}

func TestRouteCreate(t *testing.T) {
	buf := setLogBuffer()

	for i, test := range []routeTestCase{
		// errors
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "type": "sync" }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "path": "/myroute", "type": "sync" }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingPath},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingImage},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "image": "fnproject/hello", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingPath},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "image": "fnproject/hello", "path": "myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesValidationInvalidPath},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/$/routes", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrAppsValidationInvalidName},
		{datastore.NewMockInit(nil,
			[]*models.Route{
				{
					AppName: "a",
					Path:    "/myroute",
				},
			}, nil, nil,
		), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusConflict, models.ErrRoutesAlreadyExists},

		// success
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusOK, nil},
	} {
		test.run(t, i, buf)
	}
}

func TestRoutePut(t *testing.T) {
	buf := setLogBuffer()

	for i, test := range []routeTestCase{
		// errors (NOTE: this route doesn't exist yet)
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "path": "/myroute", "type": "sync" }`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingImage},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesValidationMissingImage},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "myroute", "type": "sync" } }`, http.StatusConflict, models.ErrRoutesPathImmutable},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "diffRoute", "type": "sync" } }`, http.StatusConflict, models.ErrRoutesPathImmutable},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/$/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrAppsValidationInvalidName},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "invalid-type" } }`, http.StatusBadRequest, models.ErrRoutesValidationInvalidType},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "format": "invalid-format", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesValidationInvalidFormat},

		// success
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusOK, nil},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "type": "sync" } }`, http.StatusOK, nil},
	} {
		test.run(t, i, buf)
	}
}

func TestRouteDelete(t *testing.T) {
	buf := setLogBuffer()

	routes := models.Routes{{AppName: "a", Path: "/myroute"}}
	apps := models.Apps{{Name: "a", Routes: routes, Config: nil}}

	for i, test := range []struct {
		ds            models.Datastore
		logDB         models.FnLog
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{datastore.NewMock(), logs.NewMock(), "/v1/apps/a/routes/missing", "", http.StatusNotFound, models.ErrRoutesNotFound},
		{datastore.NewMockInit(apps, routes, nil, nil), logs.NewMock(), "/v1/apps/a/routes/myroute", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, DefaultEnqueue)
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

	rnr, cancel := testRunner(t)
	defer cancel()

	ds := datastore.NewMock()
	fnl := logs.NewMock()

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, DefaultEnqueue)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps/a/routes", "", http.StatusNotFound, models.ErrAppsNotFound},
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

	rnr, cancel := testRunner(t)
	defer cancel()

	ds := datastore.NewMock()
	fnl := logs.NewMock()

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, DefaultEnqueue)

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

	for i, test := range []routeTestCase{
		// errors
		{datastore.NewMock(), logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{datastore.NewMock(), logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{}`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{datastore.NewMock(), logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "type": "invalid-type" } }`, http.StatusBadRequest, models.ErrRoutesValidationInvalidType},
		{datastore.NewMock(), logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "format": "invalid-format" } }`, http.StatusBadRequest, models.ErrRoutesValidationInvalidFormat},

		// success
		{datastore.NewMockInit(nil,
			[]*models.Route{
				{
					AppName: "a",
					Path:    "/myroute/do",
				},
			}, nil, nil,
		), logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "image": "fnproject/hello" } }`, http.StatusOK, nil},

		// Addresses #381
		{datastore.NewMockInit(nil,
			[]*models.Route{
				{
					AppName: "a",
					Path:    "/myroute/do",
				},
			}, nil, nil,
		), logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "path": "/otherpath" } }`, http.StatusConflict, models.ErrRoutesPathImmutable},
	} {
		test.run(t, i, buf)
	}
}
