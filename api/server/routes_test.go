package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	logDB         models.LogStore
	method        string
	path          string
	body          string
	expectedCode  int
	expectedError error
}

func (test *routeTestCase) run(t *testing.T, i int, buf *bytes.Buffer) {
	rnr, cancel := testRunner(t)
	srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)

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
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { } }`, http.StatusBadRequest, models.ErrRoutesMissingPath},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesMissingImage},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "image": "fnproject/hello", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesMissingPath},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/a/routes", `{ "route": { "image": "fnproject/hello", "path": "myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesInvalidPath},
		{datastore.NewMock(), logs.NewMock(), http.MethodPost, "/v1/apps/$/routes", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrAppsInvalidName},
		{datastore.NewMockInit(nil,
			[]*models.Route{
				{
					AppName: "a",
					Path:    "/myroute",
				},
			}, nil,
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
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesMissingImage},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesMissingImage},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "myroute", "type": "sync" } }`, http.StatusConflict, models.ErrRoutesPathImmutable},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "diffRoute", "type": "sync" } }`, http.StatusConflict, models.ErrRoutesPathImmutable},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/$/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusBadRequest, models.ErrAppsInvalidName},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "invalid-type" } }`, http.StatusBadRequest, models.ErrRoutesInvalidType},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "format": "invalid-format", "type": "sync" } }`, http.StatusBadRequest, models.ErrRoutesInvalidFormat},

		// success
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "path": "/myroute", "type": "sync" } }`, http.StatusOK, nil},
		{datastore.NewMock(), logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute", `{ "route": { "image": "fnproject/hello", "type": "sync" } }`, http.StatusOK, nil},
	} {
		test.run(t, i, buf)
	}
}

func TestRouteDelete(t *testing.T) {
	buf := setLogBuffer()

	routes := []*models.Route{{AppName: "a", Path: "/myroute"}}
	apps := []*models.App{{Name: "a", Config: nil}}

	for i, test := range []struct {
		ds            models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{datastore.NewMock(), logs.NewMock(), "/v1/apps/a/routes/missing", "", http.StatusNotFound, models.ErrRoutesNotFound},
		{datastore.NewMockInit(apps, routes, nil), logs.NewMock(), "/v1/apps/a/routes/myroute", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)
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

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp"},
		},
		[]*models.Route{
			{
				AppName: "myapp",
				Path:    "/myroute",
			},
			{
				AppName: "myapp",
				Path:    "/myroute1",
			},
			{
				AppName: "myapp",
				Path:    "/myroute2",
				Image:   "fnproject/hello",
			},
		},
		nil, // no calls
	)
	fnl := logs.NewMock()

	r1b := base64.RawURLEncoding.EncodeToString([]byte("/myroute"))
	r2b := base64.RawURLEncoding.EncodeToString([]byte("/myroute1"))
	r3b := base64.RawURLEncoding.EncodeToString([]byte("/myroute2"))

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path string
		body string

		expectedCode  int
		expectedError error
		expectedLen   int
		nextCursor    string
	}{
		{"/v1/apps//routes", "", http.StatusBadRequest, models.ErrAppsMissingName, 0, ""},
		{"/v1/apps/a/routes", "", http.StatusNotFound, models.ErrAppsNotFound, 0, ""},
		{"/v1/apps/myapp/routes", "", http.StatusOK, nil, 3, ""},
		{"/v1/apps/myapp/routes?per_page=1", "", http.StatusOK, nil, 1, r1b},
		{"/v1/apps/myapp/routes?per_page=1&cursor=" + r1b, "", http.StatusOK, nil, 1, r2b},
		{"/v1/apps/myapp/routes?per_page=1&cursor=" + r2b, "", http.StatusOK, nil, 1, r3b},
		{"/v1/apps/myapp/routes?per_page=100&cursor=" + r2b, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v1/apps/myapp/routes?per_page=1&cursor=" + r3b, "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
		{"/v1/apps/myapp/routes?image=fnproject/hello", "", http.StatusOK, nil, 1, ""},
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
		} else {
			// normal path

			var resp routesResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Routes) != test.expectedLen {
				t.Errorf("Test %d: Expected route length to be %d, but got %d", i, test.expectedLen, len(resp.Routes))
			}
			if resp.NextCursor != test.nextCursor {
				t.Errorf("Test %d: Expected next_cursor to be %s, but got %s", i, test.nextCursor, resp.NextCursor)
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

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

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
	ds := datastore.NewMockInit(nil, nil, nil)

	for i, test := range []routeTestCase{
		// success
		{ds, logs.NewMock(), http.MethodPut, "/v1/apps/a/routes/myroute/do", `{ "route": { "image": "fnproject/yodawg" } }`, http.StatusOK, nil},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "image": "fnproject/hello" } }`, http.StatusOK, nil},

		// errors (after success, so route exists)
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{}`, http.StatusBadRequest, models.ErrRoutesMissingNew},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "type": "invalid-type" } }`, http.StatusBadRequest, models.ErrRoutesInvalidType},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "format": "invalid-format" } }`, http.StatusBadRequest, models.ErrRoutesInvalidFormat},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "timeout": 121 } }`, http.StatusBadRequest, models.ErrRoutesInvalidTimeout},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "type": "async", "timeout": 3601 } }`, http.StatusBadRequest, models.ErrRoutesInvalidTimeout},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "type": "async", "timeout": 121, "idle_timeout": 240 } }`, http.StatusOK, nil}, // should work if async
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "idle_timeout": 3601 } }`, http.StatusBadRequest, models.ErrRoutesInvalidIdleTimeout},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "memory": 100000000000000 } }`, http.StatusBadRequest, models.ErrRoutesInvalidMemory},
		// TODO this should be correct, waiting for patch to come in
		//{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/b/routes/myroute/dont", `{ "route": {} }`, http.StatusNotFound, models.ErrAppsNotFound},
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/dont", `{ "route": {} }`, http.StatusNotFound, models.ErrRoutesNotFound},

		// Addresses #381
		{ds, logs.NewMock(), http.MethodPatch, "/v1/apps/a/routes/myroute/do", `{ "route": { "path": "/otherpath" } }`, http.StatusConflict, models.ErrRoutesPathImmutable},
	} {
		test.run(t, i, buf)
	}
}
