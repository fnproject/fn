package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

const (
	BaseRoute = "/v2/triggers"
)

func TestTriggerCreate(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	a := &models.App{ID: "appid"}
	a.SetDefaults()

	fn := &models.Fn{ID: "fnid"}
	fn.SetDefaults()
	commonDS := datastore.NewMockInit([]*models.App{a}, []*models.Fn{fn})

	for i, test := range []struct {
		mock          models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{commonDS, logs.NewMock(), BaseRoute, ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{commonDS, logs.NewMock(), BaseRoute, `{}`, http.StatusBadRequest, models.ErrMissingName},

		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "Test" }`, http.StatusBadRequest, models.ErrMissingAppID},
		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "Test", "app_id": "foo" }`, http.StatusBadRequest, models.ErrMissingFnID},
		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "Test", "app_id": "foo", "fn_id": "foo "}`, http.StatusBadRequest, models.ErrTriggerTypeUnknown},

		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "1234567890123456789012345678901" } }`, http.StatusBadRequest, models.ErrTooLongName},
		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "&&%@!#$#@$" } }`, http.StatusBadRequest, models.ErrInvalidName},
		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "trigger", "app_id": "appid", "fn_id": "fnid", "type": "HTTP", "source": "src", "annotations" : { "":"val" }}`, http.StatusBadRequest, models.ErrInvalidAnnotationKey},
		{commonDS, logs.NewMock(), BaseRoute, `{ "id": "asdasca", "name": "trigger", "app_id": "appid", "fn_id": "fnid", "type": "HTTP", "source": "src"}`, http.StatusBadRequest, models.ErrIDProvided},
		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "trigger", "app_id": "appid", "fn_id": "fnid", "type": "unsupported", "source": "src"}`, http.StatusBadRequest, models.ErrTriggerTypeUnknown},

		// // success
		{commonDS, logs.NewMock(), BaseRoute, `{ "name": "trigger", "app_id": "appid", "fn_id": "fnid", "type": "HTTP", "source": "src"}`, http.StatusOK, nil},
	} {

		rnr, cancel := testRunner(t)
		srv := testServer(test.mock, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)
		router := srv.Router

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "POST", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getV1ErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s` but got `%s`",
					i, test.expectedError.Error(), resp.Error.Message)
			}
		}

		if test.expectedCode == http.StatusOK {
			var trigger models.Trigger
			err := json.NewDecoder(rec.Body).Decode(&trigger)
			if err != nil {
				t.Log(buf.String())
				t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
			}

			// IsZero() doesn't really work, this ensures it's not unset as long as we're not in 1970
			if time.Time(trigger.CreatedAt).Before(time.Now().Add(-1 * time.Hour)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected created_at to be set on trigger, it wasn't: %s", i, trigger.CreatedAt)
			}
			if !(time.Time(trigger.CreatedAt)).Equal(time.Time(trigger.UpdatedAt)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected updated_at to be set and same as created at, it wasn't: %s %s", i, trigger.CreatedAt, trigger.UpdatedAt)
			}

			_, rec := routerRequest(t, router, "GET", BaseRoute+"/"+trigger.ID, body)

			if rec.Code != http.StatusOK {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected to be able to GET trigger after successful PUT: %d", i, rec.Code)
			}

			var triggerGet models.Trigger
			err = json.NewDecoder(rec.Body).Decode(&triggerGet)
			if err != nil {
				t.Log(buf.String())
				t.Errorf("Test %d: error decoding body for GET 'ok' json, it was a lie: %v", i, err)
			}

			if !triggerGet.Equals(&trigger) {
				t.Errorf("Test %d: GET trigger should match result of PUT trigger: %v, %v", i, triggerGet, trigger)
			}

			cancel()
		}
	}
}

func TestTriggerDelete(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	trig := &models.Trigger{
		ID: "triggerid",
	}
	trig.SetDefaults()
	ds := datastore.NewMockInit([]*models.Trigger{trig})
	for i, test := range []struct {
		ds            models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{datastore.NewMock(), logs.NewMock(), BaseRoute + "/triggerid", "", http.StatusNotFound, nil},
		{ds, logs.NewMock(), BaseRoute + "/triggerid", "", http.StatusNoContent, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)

		_, rec := routerRequest(t, srv.Router, "DELETE", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getV1ErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
		cancel()
	}
}

func TestTriggerList(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMockInit(
		[]*models.Trigger{
			{ID: "trigger1", AppID: "appid", FnID: "fnid"},
			{ID: "trigger2", AppID: "appid"},
			{ID: "trigger3", AppID: "appid"},
		},
	)
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	a1b := base64.RawURLEncoding.EncodeToString([]byte("trigger1"))
	a2b := base64.RawURLEncoding.EncodeToString([]byte("trigger2"))
	a3b := base64.RawURLEncoding.EncodeToString([]byte("trigger3"))

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
		expectedLen   int
		nextCursor    string
	}{
		{"/v2/triggers?per_page", "", http.StatusBadRequest, nil, 0, ""},
		{"/v2/triggers?app_id=appid&per_page", "", http.StatusOK, nil, 3, ""},
		{"/v2/triggers?app_id=appid&fn_id=fnid&per_page", "", http.StatusOK, nil, 1, ""},
		{"/v2/triggers?app_id=appid&per_page=1", "", http.StatusOK, nil, 1, a1b},
		{"/v2/triggers?app_id=appid&per_page=1&cursor=" + a1b, "", http.StatusOK, nil, 1, a2b},
		{"/v2/triggers?app_id=appid&per_page=1&cursor=" + a2b, "", http.StatusOK, nil, 1, a3b},
		{"/v2/triggers?app_id=appid&per_page=100&cursor=" + a2b, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v2/triggers?app_id=appid&per_page=1&cursor=" + a3b, "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getV1ErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		} else {
			// normal path

			var resp triggerListResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Items) != test.expectedLen {
				t.Errorf("Test %d: Expected apps length to be %d, but got %d", i, test.expectedLen, len(resp.Items))
			}
			if resp.NextCursor != test.nextCursor {
				t.Errorf("Test %d: Expected next_cursor to be %s, but got %s", i, test.nextCursor, resp.NextCursor)
			}
		}
	}
}

func TestTriggerGet(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	a := &models.App{ID: "appid"}
	a.SetDefaults()

	fn := &models.Fn{ID: "fnid"}
	fn.SetDefaults()

	trig := &models.Trigger{ID: "triggerid"}
	trig.SetDefaults()
	commonDS := datastore.NewMockInit([]*models.App{a}, []*models.Fn{fn}, []*models.Trigger{trig})

	for i, test := range []struct {
		mock         models.Datastore
		logDB        models.LogStore
		path         string
		expectedCode int
	}{
		{commonDS, logs.NewMock(), BaseRoute + "/notexist", http.StatusNotFound},
		{commonDS, logs.NewMock(), BaseRoute + "/triggerid", http.StatusOK},
	} {
		rnr, cancel := testRunner(t)
		defer cancel()
		srv := testServer(test.mock, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)
		router := srv.Router

		_, rec := routerRequest(t, router, "GET", test.path, bytes.NewBuffer([]byte("")))

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		var triggerGet models.Trigger
		err := json.NewDecoder(rec.Body).Decode(&triggerGet)
		if err != nil {
			t.Errorf("Test %d: Expected to decode json: %s", i, err)
		}
	}
}

func TestTriggerUpdate(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	a := &models.App{ID: "appid"}
	a.SetDefaults()

	fn := &models.Fn{ID: "fnid"}
	fn.SetDefaults()

	trig := &models.Trigger{ID: "triggerid",
		Name:   "Name",
		AppID:  "appid",
		FnID:   "fnid",
		Type:   "HTTP",
		Source: "source"}

	trig.SetDefaults()
	commonDS := datastore.NewMockInit([]*models.App{a}, []*models.Fn{fn}, []*models.Trigger{trig})

	for i, test := range []struct {
		mock          models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		name          string
		expectedCode  int
		expectedError error
	}{
		{commonDS, logs.NewMock(), BaseRoute + "/notexist", `{"id": "triggerid", "name":"changed"}`, "", http.StatusBadRequest, nil},
		{commonDS, logs.NewMock(), BaseRoute + "/notexist", `{"id": "notexist", "name":"changed"}`, "", http.StatusNotFound, nil},
		{commonDS, logs.NewMock(), BaseRoute + "/triggerid", `{"id": "nonmatching", "name":"changed}`, "", http.StatusBadRequest, models.ErrIDMismatch},
		{commonDS, logs.NewMock(), BaseRoute + "/triggerid", `{"id": "triggerid", "name":"changed"}`, "changed", http.StatusOK, nil},
		{commonDS, logs.NewMock(), BaseRoute + "/triggerid", `{"name":"again"}`, "again", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		defer cancel()
		srv := testServer(test.mock, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)
		router := srv.Router

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "PUT", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)

			if test.expectedError != nil {
				resp := getV1ErrorResponse(t, rec)
				if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
					t.Errorf("Test %d: Expected error message to have `%s` but got `%s`",
						i, test.expectedError.Error(), resp.Error.Message)
				}
			}
		}

		if rec.Code == http.StatusOK {
			_, rec := routerRequest(t, router, "GET", BaseRoute+"/triggerid", bytes.NewBuffer([]byte("")))

			var triggerGet models.Trigger
			err := json.NewDecoder(rec.Body).Decode(&triggerGet)
			if err != nil {
				t.Errorf("Test %d: Expected to decode json: %s", i, err)
			}

			trig.Name = test.name
			if !triggerGet.Equals(trig) {
				t.Errorf("Test%d: trigger should be updated: %v : %v", i, trig, triggerGet)
			}
		}
	}
}
