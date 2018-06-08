package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

type funcTestCase struct {
	ds            models.Datastore
	logDB         models.LogStore
	method        string
	path          string
	body          string
	expectedCode  int
	expectedError error
}

func (test *funcTestCase) run(t *testing.T, i int, buf *bytes.Buffer) {
	rnr, cancel := testRunner(t)
	srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)

	body := bytes.NewBuffer([]byte(test.body))
	_, rec := routerRequest(t, srv.Router, test.method, test.path, body)

	if rec.Code != test.expectedCode {
		t.Log(buf.String())
		t.Log(rec.Body.String())
		t.Errorf("Test %d: Expected status code to be %d but was %d",
			i, test.expectedCode, rec.Code)
	}

	if test.expectedError != nil {
		resp := getErrorResponse(t, rec)
		if resp.Error == nil {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected error message to have `%s`, but it was nil",
				i, test.expectedError)
		} else if resp.Error.Message != test.expectedError.Error() {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected error message to have `%s`, but it was `%s`",
				i, test.expectedError, resp.Error.Message)
		}
	}

	if test.expectedCode == http.StatusOK {
		var fwrap models.FnWrapper
		err := json.NewDecoder(rec.Body).Decode(&fwrap)
		if err != nil {
			t.Log(buf.String())
			t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
		}

		fn := fwrap.Fn
		if test.method == http.MethodPut {
			// IsZero() doesn't really work, this ensures it's not unset as long as we're not in 1970
			if time.Time(fn.CreatedAt).Before(time.Now().Add(-1 * time.Hour)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected created_at to be set on func, it wasn't: %s", i, fn.CreatedAt)
			}
			if time.Time(fn.UpdatedAt).Before(time.Now().Add(-1 * time.Hour)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected updated_at to be set on func, it wasn't: %s", i, fn.UpdatedAt)
			}
			if fn.ID == "" {
				t.Log(buf.String())
				t.Errorf("Test %d: expected id to be non-empty, it was empty: %v", i, fn)
			}
		}
	}

	cancel()
	buf.Reset()
}

func TestFnPut(t *testing.T) {
	buf := setLogBuffer()

	a := &models.App{Name: "a"}
	a.SetDefaults()
	ds := datastore.NewMockInit([]*models.App{a})
	ls := logs.NewMock()
	for i, test := range []funcTestCase{
		// errors
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ }`, http.StatusBadRequest, models.ErrFnsMissingNew},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ "image": "yo" }`, http.StatusBadRequest, models.ErrFnsMissingNew},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ "fn": { } }`, http.StatusBadRequest, models.ErrFnsMissingImage},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/ ", `{ "fn": { "image": "fnproject/fn-test-utils" } }`, http.StatusBadRequest, models.ErrFnsInvalidName},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ "fn": { "image": "fnproject/fn-test-utils", "format": "wazzup" } }`, http.StatusBadRequest, models.ErrFnsInvalidFormat},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ "fn": { "image": "fnproject/fn-test-utils", "cpus": "-100" } }`, http.StatusBadRequest, models.ErrInvalidCPUs},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ "fn": { "image": "fnproject/fn-test-utils", "timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidTimeout},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ "fn": { "image": "fnproject/fn-test-utils", "idle_timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidIdleTimeout},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/a", `{ "fn": { "image": "fnproject/fn-test-utils", "memory": 100000000000000 } }`, http.StatusBadRequest, models.ErrInvalidMemory},

		// success create & update
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "image": "fnproject/fn-test-utils" } }`, http.StatusOK, nil},

		// TODO(reed): discuss on #988 do we want to allow partial modifications still?
		// partial updates should work
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "image": "fnproject/test" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "format": "http" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "cpus": "100m" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "cpus": "0.2" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "memory": 1000 } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "timeout": 10 } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "idle_timeout": 10 } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "config": {"k":"v"} } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "annotations": {"k":"v"} } }`, http.StatusOK, nil},

		// test that partial update fails w/ same errors as create
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "format": "wazzup" } }`, http.StatusBadRequest, models.ErrFnsInvalidFormat},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "cpus": "-100" } }`, http.StatusBadRequest, models.ErrInvalidCPUs},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidTimeout},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "idle_timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidIdleTimeout},
		{ds, ls, http.MethodPut, "/v1/apps/a/fns/myfunc", `{ "fn": { "memory": 100000000000000 } }`, http.StatusBadRequest, models.ErrInvalidMemory},
	} {
		test.run(t, i, buf)
	}
}

func TestFnDelete(t *testing.T) {
	buf := setLogBuffer()

	a := &models.App{Name: "a"}
	a.SetDefaults()
	fns := []*models.Fn{{Name: "myfunc", AppID: a.ID}}
	commonDS := datastore.NewMockInit([]*models.App{a}, fns)

	for i, test := range []struct {
		ds            models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{commonDS, logs.NewMock(), "/v1/apps/a/fns/missing", "", http.StatusNotFound, models.ErrFnsNotFound},
		{commonDS, logs.NewMock(), "/v1/apps/a/fns/myfunc", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)
		_, rec := routerRequest(t, srv.Router, "DELETE", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Log(rec.Body.String())
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

func TestFnList(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancel := testRunner(t)
	defer cancel()

	// ids are sortable, need to test cursoring works as expected
	r1b := id.New().String()
	r2b := id.New().String()
	r3b := id.New().String()

	app := &models.App{Name: "myapp"}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{
			{
				ID:    r1b,
				Name:  "myfunc",
				AppID: app.ID,
				Image: "fnproject/fn-test-utils",
			},
			{
				ID:    r2b,
				Name:  "myfunc1",
				AppID: app.ID,
				Image: "fnproject/fn-test-utils",
			},
			{
				ID:    r3b,
				Name:  "myfunc2",
				AppID: app.ID,
				Image: "fnproject/yo",
			},
		},
	)
	fnl := logs.NewMock()

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path string
		body string

		expectedCode  int
		expectedError error
		expectedLen   int
		nextCursor    string
	}{
		{"/v1/apps//fns", "", http.StatusBadRequest, models.ErrAppsMissingName, 0, ""},
		{"/v1/apps/a/fns", "", http.StatusNotFound, models.ErrAppsNotFound, 0, ""},
		{"/v1/apps/myapp/fns", "", http.StatusOK, nil, 3, ""},
		{"/v1/apps/myapp/fns?per_page=1", "", http.StatusOK, nil, 1, r1b},
		{"/v1/apps/myapp/fns?per_page=1&cursor=" + r1b, "", http.StatusOK, nil, 1, r2b},
		{"/v1/apps/myapp/fns?per_page=1&cursor=" + r2b, "", http.StatusOK, nil, 1, r3b},
		{"/v1/apps/myapp/fns?per_page=100&cursor=" + r2b, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v1/apps/myapp/fns?per_page=1&cursor=" + r3b, "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
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

			var resp fnsResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Fns) != test.expectedLen {
				t.Errorf("Test %d: Expected fns length to be %d, but got %d", i, test.expectedLen, len(resp.Fns))
			}
			if resp.NextCursor != test.nextCursor {
				t.Errorf("Test %d: Expected next_cursor to be %s, but got %s", i, test.nextCursor, resp.NextCursor)
			}
		}
	}
}

func TestFnGet(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancel := testRunner(t)
	defer cancel()

	app := &models.App{Name: "myapp"}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{
			{
				Name:  "myfunc",
				Image: "fnproject/fn-test-utils",
			},
		})
	fnl := logs.NewMock()

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps//fns/myfunc", "", http.StatusBadRequest, models.ErrAppsMissingName},
		{"/v1/apps/a/fns/myfunc", "", http.StatusNotFound, models.ErrAppsNotFound},
		{"/v1/apps/myapp/fns/missing", "", http.StatusNotFound, models.ErrFnsNotFound},
		{"/v1/apps/myapp/fns/myfunc", "", http.StatusOK, nil},
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
