package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func setLogBuffer() *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte('\n')
	logrus.SetOutput(&buf)
	gin.DefaultErrorWriter = &buf
	gin.DefaultWriter = &buf
	log.SetOutput(&buf)
	return &buf
}

func TestAppCreate(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	for i, test := range []struct {
		mock          models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{}`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "name": "Test" }`, http.StatusBadRequest, models.ErrAppsMissingNew},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "" } }`, http.StatusBadRequest, models.ErrAppsMissingName},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "1234567890123456789012345678901" } }`, http.StatusBadRequest, models.ErrAppsTooLongName},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusBadRequest, models.ErrAppsInvalidName},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "&&%@!#$#@$" } }`, http.StatusBadRequest, models.ErrAppsInvalidName},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "app", "metadata" : { "":"val" }}}`, http.StatusBadRequest, models.ErrInvalidMetadataKey},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "app", "metadata" : { "key":"" }}}`, http.StatusBadRequest, models.ErrInvalidMetadataValue},
		// success
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "teste" } }`, http.StatusOK, nil},
		{datastore.NewMock(), logs.NewMock(), "/v1/apps", `{ "app": { "name": "teste" , "metadata": {"k1":"v1", "k2":[]}}}`, http.StatusOK, nil},
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
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s` but got `%s`",
					i, test.expectedError.Error(), resp.Error.Message)
			}
		}

		if test.expectedCode == http.StatusOK {
			var awrap models.AppWrapper
			err := json.NewDecoder(rec.Body).Decode(&awrap)
			if err != nil {
				t.Log(buf.String())
				t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
			}

			app := awrap.App

			// IsZero() doesn't really work, this ensures it's not unset as long as we're not in 1970
			if time.Time(app.CreatedAt).Before(time.Now().Add(-1 * time.Hour)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected created_at to be set on app, it wasn't: %s", i, app.CreatedAt)
			}
			if !(time.Time(app.CreatedAt)).Equal(time.Time(app.UpdatedAt)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected updated_at to be set and same as created at, it wasn't: %s %s", i, app.CreatedAt, app.UpdatedAt)
			}
		}

		cancel()
	}
}

func TestAppDelete(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	for i, test := range []struct {
		ds            models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{datastore.NewMock(), logs.NewMock(), "/v1/apps/myapp", "", http.StatusNotFound, nil},
		{datastore.NewMockInit(
			[]*models.App{{
				Name: "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", "", http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)

		_, rec := routerRequest(t, srv.Router, "DELETE", test.path, nil)

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
		cancel()
	}
}

func TestAppList(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp"},
			{Name: "myapp2"},
			{Name: "myapp3"},
		},
		nil, // no routes
		nil, // no calls
	)
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	a1b := base64.RawURLEncoding.EncodeToString([]byte("myapp"))
	a2b := base64.RawURLEncoding.EncodeToString([]byte("myapp2"))
	a3b := base64.RawURLEncoding.EncodeToString([]byte("myapp3"))

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
		expectedLen   int
		nextCursor    string
	}{
		{"/v1/apps?per_page", "", http.StatusOK, nil, 3, ""},
		{"/v1/apps?per_page=1", "", http.StatusOK, nil, 1, a1b},
		{"/v1/apps?per_page=1&cursor=" + a1b, "", http.StatusOK, nil, 1, a2b},
		{"/v1/apps?per_page=1&cursor=" + a2b, "", http.StatusOK, nil, 1, a3b},
		{"/v1/apps?per_page=100&cursor=" + a2b, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v1/apps?per_page=1&cursor=" + a3b, "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

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
		} else {
			// normal path

			var resp appsResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Apps) != test.expectedLen {
				t.Errorf("Test %d: Expected apps length to be %d, but got %d", i, test.expectedLen, len(resp.Apps))
			}
			if resp.NextCursor != test.nextCursor {
				t.Errorf("Test %d: Expected next_cursor to be %s, but got %s", i, test.nextCursor, resp.NextCursor)
			}
		}
	}
}

func TestAppGet(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

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
		{"/v1/apps/myapp", "", http.StatusNotFound, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

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
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()


	for i, test := range []struct {
		mock          models.Datastore
		logDB         models.LogStore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{datastore.NewMock(), logs.NewMock(), "/v1/apps/myapp", ``, http.StatusBadRequest, models.ErrInvalidJSON},

		// Addresses #380
		{datastore.NewMockInit(
			[]*models.App{{
				Name: "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", `{ "app": { "name": "othername" } }`, http.StatusConflict, nil},

		// success: add/set MD key
		{datastore.NewMockInit(
			[]*models.App{{
				Name:     "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", `{ "app": { "metadata": {"k-0" : "val"} } }`, http.StatusOK, nil},

		// success
		{datastore.NewMockInit(
			[]*models.App{{
				Name: "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", `{ "app": { "config": { "test": "1" } } }`, http.StatusOK, nil},

		// success
		{datastore.NewMockInit(
			[]*models.App{{
				Name: "myapp",
			}}, nil, nil,
		), logs.NewMock(), "/v1/apps/myapp", `{ "app": { "config": { "test": "1" } } }`, http.StatusOK, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.mock, &mqs.Mock{}, test.logDB, rnr, ServerTypeFull)

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, srv.Router, "PATCH", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s` but was `%s`",
					i, test.expectedError.Error(),resp.Error.Message)
			}
		}

		if test.expectedCode == http.StatusOK {
			var awrap models.AppWrapper
			err := json.NewDecoder(rec.Body).Decode(&awrap)
			if err != nil {
				t.Log(buf.String())
				t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
			}

			app := awrap.App
			// IsZero() doesn't really work, this ensures it's not unset as long as we're not in 1970
			if time.Time(app.UpdatedAt).Before(time.Now().Add(-1 * time.Hour)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected updated_at to be set on app, it wasn't: %s", i, app.UpdatedAt)
			}

			// this isn't perfect, since a PATCH could succeed without updating any
			// fields (among other reasons), but just don't make a test for that or
			// special case (the body or smth) to ignore it here!
			// this is a decent approximation that the timestamp gets changed
			if (time.Time(app.UpdatedAt)).Equal(time.Time(app.CreatedAt)) {
				t.Log(buf.String())
				t.Errorf("Test %d: expected updated_at to not be the same as created at, it wasn't: %s %s", i, app.CreatedAt, app.UpdatedAt)
			}
		}

		cancel()
	}
}
