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
		var fwrap models.FuncWrapper
		err := json.NewDecoder(rec.Body).Decode(&fwrap)
		if err != nil {
			t.Log(buf.String())
			t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
		}

		fn := fwrap.Func
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

func TestFuncPut(t *testing.T) {
	buf := setLogBuffer()

	ds := datastore.NewMockInit()
	ls := logs.NewMock()
	for i, test := range []funcTestCase{
		// errors
		{ds, ls, http.MethodPut, "/v1/funcs/a", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ }`, http.StatusBadRequest, models.ErrFuncsMissingNew},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ "image": "yo" }`, http.StatusBadRequest, models.ErrFuncsMissingNew},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ "func": { } }`, http.StatusBadRequest, models.ErrFuncsMissingImage},
		{ds, ls, http.MethodPut, "/v1/funcs/ ", `{ "func": { "image": "fnproject/fn-test-utils" } }`, http.StatusBadRequest, models.ErrFuncsInvalidName},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ "func": { "image": "fnproject/fn-test-utils", "format": "wazzup" } }`, http.StatusBadRequest, models.ErrFuncsInvalidFormat},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ "func": { "image": "fnproject/fn-test-utils", "cpus": "-100" } }`, http.StatusBadRequest, models.ErrInvalidCPUs},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ "func": { "image": "fnproject/fn-test-utils", "timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidTimeout},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ "func": { "image": "fnproject/fn-test-utils", "idle_timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidIdleTimeout},
		{ds, ls, http.MethodPut, "/v1/funcs/a", `{ "func": { "image": "fnproject/fn-test-utils", "memory": 100000000000000 } }`, http.StatusBadRequest, models.ErrInvalidMemory},

		// success create & update
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "image": "fnproject/fn-test-utils" } }`, http.StatusOK, nil},

		// TODO(reed): discuss on #988 do we want to allow partial modifications still?
		// partial updates should work
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "image": "fnproject/test" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "format": "http" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "cpus": "100m" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "cpus": "0.2" } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "memory": 1000 } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "timeout": 10 } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "idle_timeout": 10 } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "config": {"k":"v"} } }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "annotations": {"k":"v"} } }`, http.StatusOK, nil},

		// test that partial update fails w/ same errors as create
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "format": "wazzup" } }`, http.StatusBadRequest, models.ErrFuncsInvalidFormat},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "cpus": "-100" } }`, http.StatusBadRequest, models.ErrInvalidCPUs},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidTimeout},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "idle_timeout": 3601 } }`, http.StatusBadRequest, models.ErrInvalidIdleTimeout},
		{ds, ls, http.MethodPut, "/v1/funcs/myfunc", `{ "func": { "memory": 100000000000000 } }`, http.StatusBadRequest, models.ErrInvalidMemory},
	} {
		test.run(t, i, buf)
	}
}

func TestFuncDelete(t *testing.T) {
	buf := setLogBuffer()

	funcs := []*models.Func{{Name: "myfunc"}}
	commonDS := datastore.NewMockInit(funcs)

	for i, test := range []struct {
		ds            models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{commonDS, logs.NewMock(), "/v1/funcs/missing", "", http.StatusNotFound, models.ErrFuncsNotFound},
		{commonDS, logs.NewMock(), "/v1/funcs/myfunc", "", http.StatusOK, nil},
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

func TestFuncList(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancel := testRunner(t)
	defer cancel()

	// ids are sortable, need to test cursoring works as expected
	r1b := id.New().String()
	r2b := id.New().String()
	r3b := id.New().String()

	ds := datastore.NewMockInit(
		[]*models.Func{
			{
				ID:    r1b,
				Name:  "myfunc",
				Image: "fnproject/fn-test-utils",
			},
			{
				ID:    r2b,
				Name:  "myfunc1",
				Image: "fnproject/fn-test-utils",
			},
			{
				ID:    r3b,
				Name:  "myfunc2",
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
		{"/v1/funcs", "", http.StatusOK, nil, 3, ""},
		{"/v1/funcs?per_page=1", "", http.StatusOK, nil, 1, r1b},
		{"/v1/funcs?per_page=1&cursor=" + r1b, "", http.StatusOK, nil, 1, r2b},
		{"/v1/funcs?per_page=1&cursor=" + r2b, "", http.StatusOK, nil, 1, r3b},
		{"/v1/funcs?per_page=100&cursor=" + r2b, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v1/funcs?per_page=1&cursor=" + r3b, "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
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

			var resp funcsResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Funcs) != test.expectedLen {
				t.Errorf("Test %d: Expected funcs length to be %d, but got %d", i, test.expectedLen, len(resp.Funcs))
			}
			if resp.NextCursor != test.nextCursor {
				t.Errorf("Test %d: Expected next_cursor to be %s, but got %s", i, test.nextCursor, resp.NextCursor)
			}
		}
	}
}

func TestFuncGet(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancel := testRunner(t)
	defer cancel()

	ds := datastore.NewMockInit([]*models.Func{
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
		{"/v1/funcs/missing", "", http.StatusNotFound, models.ErrFuncsNotFound},
		{"/v1/funcs/myfunc", "", http.StatusOK, nil},
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
