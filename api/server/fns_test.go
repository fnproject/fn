package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

type funcTestCase struct {
	ds            models.Datastore
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
		t.Fatalf("Test %d: Expected status code to be %d but was %d",
			i, test.expectedCode, rec.Code)
	}

	if test.expectedError != nil {
		resp := getErrorResponse(t, rec)
		if resp == nil {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected error message to have `%s`, but it was nil",
				i, test.expectedError)
		} else if resp.Message != test.expectedError.Error() {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected error message to have `%s`, but it was `%s`",
				i, test.expectedError, resp.Message)
		}
	}

	if test.expectedCode == http.StatusOK {
		var fn models.Fn
		err := json.NewDecoder(rec.Body).Decode(&fn)
		if err != nil {
			t.Log(buf.String())
			t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
		}

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

func TestFnCreate(t *testing.T) {
	buf := setLogBuffer()

	a := &models.App{Name: "a", ID: "aid"}
	ds := datastore.NewMockInit([]*models.App{a})
	for i, test := range []funcTestCase{
		// errors
		{ds, http.MethodPost, "/v2/fns", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{ds, http.MethodPost, "/v2/fns", `{ }`, http.StatusBadRequest, models.ErrFnsMissingAppID},
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s" }`, a.ID), http.StatusBadRequest, models.ErrFnsMissingName},
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s", "name": "a" }`, a.ID), http.StatusBadRequest, models.ErrFnsMissingImage},
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s", "name": " ", "image": "fnproject/fn-test-utils" }`, a.ID), http.StatusBadRequest, models.ErrFnsInvalidName},
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s", "name": "a", "image": "fnproject/fn-test-utils", "timeout": 3601 }`, a.ID), http.StatusBadRequest, models.ErrFnsInvalidTimeout},
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s", "name": "a", "image": "fnproject/fn-test-utils", "idle_timeout": 3601 }`, a.ID), http.StatusBadRequest, models.ErrFnsInvalidIdleTimeout},
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s", "name": "a", "image": "fnproject/fn-test-utils", "memory": 100000000000000 }`, a.ID), http.StatusBadRequest, models.ErrInvalidMemory},

		// success create & update
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s", "name": "myfunc", "image": "fnproject/fn-test-utils" }`, a.ID), http.StatusOK, nil},
		{ds, http.MethodPost, "/v2/fns", fmt.Sprintf(`{ "app_id": "%s", "name": "myfunc", "image": "fnproject/fn-test-utils" }`, a.ID), http.StatusConflict, models.ErrFnsExists},
	} {
		test.run(t, i, buf)
	}
}

func TestFnUpdate(t *testing.T) {
	buf := setLogBuffer()

	a := &models.App{Name: "a", ID: "app_id"}
	f := &models.Fn{ID: "fn_id", Name: "f", AppID: a.ID}
	f.SetDefaults()
	ds := datastore.NewMockInit([]*models.App{a}, []*models.Fn{f})
	ls := logs.NewMock()

	for i, test := range []funcTestCase{
		{ds, ls, http.MethodPut, "/v2/fns/missing", `{ }`, http.StatusNotFound, models.ErrFnsNotFound},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "id": "nottheid" }`, http.StatusBadRequest, models.ErrFnsIDMismatch},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "image": "fnproject/test" }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "memory": 1000 }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "timeout": 10 }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "idle_timeout": 10 }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "config": {"k":"v"} }`, http.StatusOK, nil},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "annotations": {"k":"v"} }`, http.StatusOK, nil},

		// test that partial update fails w/ same errors as create
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "timeout": 3601 }`, http.StatusBadRequest, models.ErrFnsInvalidTimeout},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "idle_timeout": 3601 }`, http.StatusBadRequest, models.ErrFnsInvalidIdleTimeout},
		{ds, ls, http.MethodPut, fmt.Sprintf("/v2/fns/%s", f.ID), `{ "memory": 100000000000000 }`, http.StatusBadRequest, models.ErrInvalidMemory},
	} {
		test.run(t, i, buf)
	}
}

func TestFnDelete(t *testing.T) {
	buf := setLogBuffer()

	a := &models.App{Name: "a", ID: "appid"}
	f := &models.Fn{ID: "fn_id", Name: "myfunc", AppID: a.ID}
	f.SetDefaults()
	commonDS := datastore.NewMockInit([]*models.App{a}, []*models.Fn{f})

	for i, test := range []struct {
		ds            models.Datastore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{commonDS, "/v2/fns/missing", "", http.StatusNotFound, models.ErrFnsNotFound},
		{commonDS, fmt.Sprintf("/v2/fns/%s", f.ID), "", http.StatusNoContent, nil},
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

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
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
	r4b := id.New().String()

	fn1 := "myfunc1"
	fn2 := "myfunc2"
	fn3 := "myfunc3"
	fn4 := "myfunc3"

	app1 := &models.App{Name: "myapp1", ID: "app_id1"}
	app2 := &models.App{Name: "myapp2", ID: "app_id2"}
	ds := datastore.NewMockInit(
		[]*models.App{app1, app2},
		[]*models.Fn{
			{
				ID:    r1b,
				Name:  fn1,
				AppID: app1.ID,
				Image: "fnproject/fn-test-utils",
			},
			{
				ID:    r2b,
				Name:  fn2,
				AppID: app1.ID,
				Image: "fnproject/fn-test-utils",
			},
			{
				ID:    r3b,
				Name:  fn3,
				AppID: app1.ID,
				Image: "fnproject/yo",
			},
			{
				ID:    r4b,
				Name:  fn4,
				AppID: app2.ID,
				Image: "fnproject/foo",
			},
		},
	)
	srv := testServer(ds, rnr, ServerTypeFull)

	fn1b := base64.RawURLEncoding.EncodeToString([]byte(fn1))
	fn2b := base64.RawURLEncoding.EncodeToString([]byte(fn2))
	fn3b := base64.RawURLEncoding.EncodeToString([]byte(fn3))

	for i, test := range []struct {
		path string
		body string

		expectedCode  int
		expectedError error
		expectedLen   int
		nextCursor    string
	}{
		{"/v2/fns", "", http.StatusBadRequest, models.ErrFnsMissingAppID, 0, ""},
		{fmt.Sprintf("/v2/fns?app_id=%s", app1.ID), "", http.StatusOK, nil, 3, ""},
		{fmt.Sprintf("/v2/fns?app_id=%s&per_page=1", app1.ID), "", http.StatusOK, nil, 1, fn1b},
		{fmt.Sprintf("/v2/fns?app_id=%s&per_page=1&cursor=%s", app1.ID, fn1b), "", http.StatusOK, nil, 1, fn2b},
		{fmt.Sprintf("/v2/fns?app_id=%s&per_page=1&cursor=%s", app1.ID, fn2b), "", http.StatusOK, nil, 1, fn3b},
		{fmt.Sprintf("/v2/fns?app_id=%s&per_page=100&cursor=%s", app1.ID, fn3b), "", http.StatusOK, nil, 0, ""}, // cursor is empty if per_page > len(results)
		{fmt.Sprintf("/v2/fns?app_id=%s&per_page=1&cursor=%s", app1.ID, fn3b), "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		} else {
			// normal path

			var resp models.FnList
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Items) != test.expectedLen {
				t.Errorf("Test %d: Expected fns length to be %d, but got %d", i, test.expectedLen, len(resp.Items))
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

	app := &models.App{Name: "myapp", ID: "appid"}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{
			{

				ID:    "myfnId",
				Name:  "myfunc",
				AppID: "appid",
				Image: "fnproject/fn-test-utils",
			},
		})
	nilFn := new(models.Fn)

	expectedFn := &models.Fn{
		ID:    "myfnId",
		Name:  "myfunc",
		Image: "fnproject/fn-test-utils"}

	srv := testServer(ds, rnr, ServerTypeFull)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
		desiredFn     *models.Fn
	}{
		{"/v2/fns/missing", "", http.StatusNotFound, models.ErrFnsNotFound, nilFn},
		{"/v2/fns/myfnId", "", http.StatusOK, nil, expectedFn},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Fatalf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`, got `%s`",
					i, test.expectedError.Error(), resp.Message)
			}
		}

		if !test.desiredFn.Equals(nilFn) {
			var fn models.Fn
			err := json.NewDecoder(rec.Body).Decode(&fn)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
			}
			if test.desiredFn.Equals(&fn) {
				t.Errorf("Test %d: Expected fn [%v] got [%v]", i, test.desiredFn, fn)
			}
		}
	}
}

func TestFnInvokeEndpointAnnotations(t *testing.T) {
	a := &models.App{ID: "app_id", Name: "myapp"}
	fn := &models.Fn{AppID: a.ID, Name: "fnname", Image: "fnproject/image"}

	commonDS := datastore.NewMockInit([]*models.App{a})

	srv := testServer(commonDS, nil, ServerTypeAPI)
	body, err := json.Marshal(fn)
	if err != nil {
		t.Fatalf("Failed to marshal json to create fn %s", err)
	}

	_, createFN := routerRequest(t, srv.Router, "POST", "/v2/fns", bytes.NewReader(body))
	if createFN.Code != http.StatusOK {
		t.Fatalf("expected code %d != 200 %s", createFN.Code, createFN.Body.String())
	}

	var fnCreate models.Fn
	err = json.NewDecoder(createFN.Body).Decode(&fnCreate)
	if err != nil {
		t.Fatalf("Invalid json from server %s", err)
	}

	const fnEndpoint = "fnproject.io/fn/invokeEndpoint"
	v, err := fnCreate.Annotations.GetString(fnEndpoint)
	if err != nil {
		t.Errorf("failed to get fn %s", err)
	}
	if v != "http://127.0.0.1:8080/invoke/"+fnCreate.ID {
		t.Errorf("unexpected fn val %s", v)
	}

	_, rec := routerRequest(t, srv.Router, "GET", "/v2/fns/"+fnCreate.ID, bytes.NewBuffer([]byte("")))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected code %d != 200", rec.Code)
	}
	var fnGet models.Fn
	err = json.NewDecoder(rec.Body).Decode(&fnGet)
	if err != nil {
		t.Fatalf("Invalid json from server %s", err)
	}

	v, err = fnGet.Annotations.GetString(fnEndpoint)
	if err != nil {
		t.Fatalf("failed to get fn %s", err)
	}
	if v != "http://127.0.0.1:8080/invoke/"+fnCreate.ID {
		t.Errorf("unexpected fn val %s", v)
	}

	_, rec = routerRequest(t, srv.Router, "GET", fmt.Sprintf("/v2/fns?app_id=%s", a.ID), nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected code %d != 200", rec.Code)
	}

	var resp models.FnList
	err = json.NewDecoder(rec.Body).Decode(&resp)
	if err != nil {
		t.Fatalf("Invalid json from server %s : %s", err, string(rec.Body.Bytes()))
	}

	if len(resp.Items) != 1 {
		t.Fatalf("Unexpected fn list result, %v", resp)
	}

	v, err = resp.Items[0].Annotations.GetString(fnEndpoint)
	if v != "http://127.0.0.1:8080/invoke/"+fnCreate.ID {
		t.Errorf("unexpected fn val %s", v)
	}
}
