package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"fmt"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/models"
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
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{datastore.NewMock(), "/v2/apps", ``, http.StatusBadRequest, models.ErrInvalidJSON},
		{datastore.NewMock(), "/v2/apps", `{}`, http.StatusBadRequest, models.ErrMissingName},
		{datastore.NewMock(), "/v2/apps", `{"name": "app", "id":"badId"}`, http.StatusBadRequest, models.ErrAppIDProvided},
		{datastore.NewMock(), "/v2/apps", `{ "name": "" }`, http.StatusBadRequest, models.ErrMissingName},
		{datastore.NewMock(), "/v2/apps", `{"name": "1234567890123456789012345678901" }`, http.StatusBadRequest, models.ErrAppsTooLongName},
		{datastore.NewMock(), "/v2/apps", `{ "name": "&&%@!#$#@$" }`, http.StatusBadRequest, models.ErrAppsInvalidName},
		{datastore.NewMock(), "/v2/apps", `{ "name": "app", "annotations" : { "":"val" }}`, http.StatusBadRequest, models.ErrInvalidAnnotationKey},
		{datastore.NewMock(), "/v2/apps", `{"name": "app", "annotations" : { "key":"" }}`, http.StatusBadRequest, models.ErrInvalidAnnotationValue},
		{datastore.NewMock(), "/v2/apps", `{ "name": "app", "syslog_url":"yo"}`, http.StatusBadRequest, errors.New(`invalid syslog url: "yo"`)},
		{datastore.NewMock(), "/v2/apps", `{"name": "app", "syslog_url":"yo://sup.com:1"}`, http.StatusBadRequest, errors.New(`invalid syslog url: "yo://sup.com:1" invalid scheme, only [tcp, udp, unix, unixgram, tcp+tls] are supported`)},
		// success
		{datastore.NewMock(), "/v2/apps", `{ "name": "teste"  }`, http.StatusOK, nil},
		{datastore.NewMock(), "/v2/apps", `{  "name": "teste" , "annotations": {"k1":"v1", "k2":[]}}`, http.StatusOK, nil},
		{datastore.NewMock(), "/v2/apps", `{"name": "teste", "syslog_url":"tcp://example.com:443" } `, http.StatusOK, nil},
		{datastore.NewMockInit([]*models.App{&models.App{ID: "appid", Name: "teste"}}), "/v2/apps", `{ "name": "teste"  }`, http.StatusConflict, models.ErrAppsAlreadyExists},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.mock, rnr, ServerTypeFull)
		router := srv.Router

		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, router, "POST", test.path, body)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s` but got `%s`",
					i, test.expectedError.Error(), resp.Message)
			}
		}

		if test.expectedCode == http.StatusOK {
			var app models.App
			err := json.NewDecoder(rec.Body).Decode(&app)
			if err != nil {
				t.Log(buf.String())
				t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
			}

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

	app := &models.App{
		Name: "myapp",
		ID:   "appId",
	}
	ds := datastore.NewMockInit([]*models.App{app})
	for i, test := range []struct {
		ds            models.Datastore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{datastore.NewMock(), "/v2/apps/myapp", "", http.StatusNotFound, nil},
		{ds, "/v2/apps/appId", "", http.StatusNoContent, nil},
	} {
		rnr, cancel := testRunner(t)
		srv := testServer(test.ds, rnr, ServerTypeFull)

		_, rec := routerRequest(t, srv.Router, "DELETE", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
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
	)
	srv := testServer(ds, rnr, ServerTypeFull)

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
		{"/v2/apps?per_page", "", http.StatusOK, nil, 3, ""},
		{"/v2/apps?per_page=1", "", http.StatusOK, nil, 1, a1b},
		{"/v2/apps?per_page=1&cursor=" + a1b, "", http.StatusOK, nil, 1, a2b},
		{"/v2/apps?per_page=1&cursor=" + a2b, "", http.StatusOK, nil, 1, a3b},
		{"/v2/apps?per_page=100&cursor=" + a2b, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v2/apps?per_page=1&cursor=" + a3b, "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		} else {
			// normal path

			var resp models.AppList
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

func TestAppGet(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	rnr, cancel := testRunner(t)
	defer cancel()
	app := &models.App{
		ID:   "appId",
		Name: "app",
	}
	ds := datastore.NewMockInit([]*models.App{app})
	srv := testServer(ds, rnr, ServerTypeFull)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v2/apps/unknownApp", "", http.StatusNotFound, models.ErrAppsNotFound},
		{"/v2/apps/appId", "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
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

	app := &models.App{
		Name: "myapp",
		ID:   "appId",
	}
	ds := datastore.NewMockInit([]*models.App{app})

	for i, test := range []struct {
		mock          models.Datastore
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		// errors
		{ds, "/v2/apps/not_app", `{ }`, http.StatusNotFound, models.ErrAppsNotFound},

		{ds, "/v2/apps/appId", ``, http.StatusBadRequest, models.ErrInvalidJSON},

		// Addresses #380
		{ds, "/v2/apps/appId", `{  "name": "othername" }`, http.StatusConflict, models.ErrAppsNameImmutable},

		// success: add/set MD key
		{ds, "/v2/apps/appId", `{ "annotations":{"foo":"bar"}}`, http.StatusOK, nil},

		// success
		{ds, "/v2/apps/appId", `{  "config": { "test": "1" }  }`, http.StatusOK, nil},

		// success
		{ds, "/v2/apps/appId", `{  "config": { "test": "1" } }`, http.StatusOK, nil},

		// success
		{ds, "/v2/apps/appId", `{ "syslog_url":"tcp://example.com:443" }`, http.StatusOK, nil},
	} {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			rnr, cancel := testRunner(t)
			defer cancel()
			srv := testServer(test.mock, rnr, ServerTypeFull)

			body := bytes.NewBuffer([]byte(test.body))
			_, rec := routerRequest(t, srv.Router, "PUT", test.path, body)

			if rec.Code != test.expectedCode {
				t.Fatalf("Test %d: Expected status code to be %d but was %d",
					i, test.expectedCode, rec.Code)
			}

			if test.expectedError != nil {
				resp := getErrorResponse(t, rec)

				if !strings.Contains(resp.Message, test.expectedError.Error()) {
					t.Errorf("Test %d: Expected error message to have `%s` but was `%s`",
						i, test.expectedError.Error(), resp.Message)
				}
			}

			if test.expectedCode == http.StatusOK {
				var app models.App
				err := json.NewDecoder(rec.Body).Decode(&app)
				if err != nil {
					t.Log(buf.String())
					t.Errorf("Test %d: error decoding body for 'ok' json, it was a lie: %v", i, err)
				}

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

		})

	}
}
