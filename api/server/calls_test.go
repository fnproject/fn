package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/go-openapi/strfmt"
)

func TestCallGet(t *testing.T) {
	buf := setLogBuffer()

	app := &models.App{Name: "myapp"}
	app.SetDefaults()
	call := &models.Call{
		AppID: app.ID,
		CallBase: models.CallBase{
			ID:    id.New().String(),
			Path:  "/thisisatest",
			Image: "fnproject/hello",
			// Delay: 0,
			Type:   "sync",
			Format: "default",
			// Payload: TODO,
			Priority:    new(int32), // TODO this is crucial, apparently
			Timeout:     30,
			IdleTimeout: 30,
			Memory:      256,
			CreatedAt:   strfmt.DateTime(time.Now()),
			URL:         "http://localhost:8080/r/myapp/thisisatest",
			Method:      "GET",
		}}

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		nil,
		[]*models.Call{call},
	)
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v1/apps//calls/" + call.ID, "", http.StatusBadRequest, models.ErrAppsMissingName},
		{"/v1/apps/nodawg/calls/" + call.ID, "", http.StatusNotFound, models.ErrAppsNotFound}, // TODO a little weird
		{"/v1/apps/myapp/calls/" + call.ID[:3], "", http.StatusNotFound, models.ErrCallNotFound},
		{"/v1/apps/myapp/calls/" + call.ID, "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

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
				t.Log(resp.Error.Message)
				t.Log(rec.Body.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
		// TODO json parse the body and assert fields
	}
}

func TestCallList(t *testing.T) {
	buf := setLogBuffer()

	app := &models.App{Name: "myapp"}
	app.SetDefaults()

	call := &models.Call{
		AppID: app.ID,
		CallBase: models.CallBase{
			ID:    id.New().String(),
			Path:  "/thisisatest",
			Image: "fnproject/hello",
			// Delay: 0,
			Type:   "sync",
			Format: "default",
			// Payload: TODO,
			Priority:    new(int32), // TODO this is crucial, apparently
			Timeout:     30,
			IdleTimeout: 30,
			Memory:      256,
			CreatedAt:   strfmt.DateTime(time.Now()),
			URL:         "http://localhost:8080/r/myapp/thisisatest",
			Method:      "GET",
		}}
	c2 := *call
	c3 := *call
	c2.ID = id.New().String()
	c2.CreatedAt = strfmt.DateTime(time.Now().Add(100 * time.Second))
	c2.Path = "test2"
	c3.ID = id.New().String()
	c3.CreatedAt = strfmt.DateTime(time.Now().Add(200 * time.Second))
	c3.Path = "/test3"

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		nil,
		[]*models.Call{call, &c2, &c3},
	)
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	// add / sub 1 second b/c unix time will lop off millis and mess up our comparisons
	rangeTest := fmt.Sprintf("from_time=%d&to_time=%d",
		time.Time(call.CreatedAt).Add(1*time.Second).Unix(),
		time.Time(c3.CreatedAt).Add(-1*time.Second).Unix(),
	)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
		expectedLen   int
		nextCursor    string
	}{
		{"/v1/apps//calls", "", http.StatusBadRequest, models.ErrAppsMissingName, 0, ""},
		{"/v1/apps/nodawg/calls", "", http.StatusNotFound, models.ErrAppsNotFound, 0, ""},
		{"/v1/apps/myapp/calls", "", http.StatusOK, nil, 3, ""},
		{"/v1/apps/myapp/calls?per_page=1", "", http.StatusOK, nil, 1, c3.ID},
		{"/v1/apps/myapp/calls?per_page=1&cursor=" + c3.ID, "", http.StatusOK, nil, 1, c2.ID},
		{"/v1/apps/myapp/calls?per_page=1&cursor=" + c2.ID, "", http.StatusOK, nil, 1, call.ID},
		{"/v1/apps/myapp/calls?per_page=100&cursor=" + c2.ID, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v1/apps/myapp/calls?per_page=1&cursor=" + call.ID, "", http.StatusOK, nil, 0, ""}, // cursor could point to empty page
		{"/v1/apps/myapp/calls?" + rangeTest, "", http.StatusOK, nil, 1, ""},
		{"/v1/apps/myapp/calls?from_time=xyz", "", http.StatusBadRequest, models.ErrInvalidFromTime, 0, ""},
		{"/v1/apps/myapp/calls?to_time=xyz", "", http.StatusBadRequest, models.ErrInvalidToTime, 0, ""},

		// TODO path isn't url safe w/ '/', so this is weird. hack in for tests
		{"/v1/apps/myapp/calls?path=test2", "", http.StatusOK, nil, 1, ""},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if resp.Error == nil || !strings.Contains(resp.Error.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`, got: `%s`",
					i, test.expectedError.Error(), resp.Error)
			}
		} else {
			// normal path

			var resp callsResponse
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Calls) != test.expectedLen {
				t.Fatalf("Test %d: Expected apps length to be %d, but got %d", i, test.expectedLen, len(resp.Calls))
			}
			if resp.NextCursor != test.nextCursor {
				t.Errorf("Test %d: Expected next_cursor to be %s, but got %s", i, test.nextCursor, resp.NextCursor)
			}
		}
	}
}
