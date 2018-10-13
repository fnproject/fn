package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

func TestCallGet(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	fn := &models.Fn{Name: "myfn", ID: "fn_id"}
	call := &models.Call{
		FnID:  fn.ID,
		ID:    id.New().String(),
		Image: "fnproject/fn-test-utils",
		// Delay: 0,
		Type: "sync",
		// Payload: TODO,
		Priority:    new(int32), // TODO this is crucial, apparently
		Timeout:     30,
		IdleTimeout: 30,
		Memory:      256,
		CreatedAt:   common.DateTime(time.Now()),
		URL:         "http://localhost:8080/r/myapp/thisisatest",
		Method:      "GET",
	}

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMockInit(
		[]*models.Fn{fn},
	)
	fnl := logs.NewMock([]*models.Call{call})
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/v2/fns//calls/" + call.ID, "", http.StatusBadRequest, models.ErrFnsMissingID},
		{"/v2/fns/missing_fn/calls/" + call.ID, "", http.StatusNotFound, models.ErrFnsNotFound},
		{"/v2/fns/fn_id/calls/" + id.New().String(), "", http.StatusNotFound, models.ErrCallNotFound},
		{"/v2/fns/fn_id/calls/" + call.ID[:3], "", http.StatusNotFound, models.ErrCallNotFound},
		{"/v2/fns/fn_id/calls/" + call.ID, "", http.StatusOK, nil},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(rec.Body.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Log(resp.Message)
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
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()

	fn := &models.Fn{ID: "fn_id"}

	call := &models.Call{
		FnID:  fn.ID,
		ID:    id.New().String(),
		Image: "fnproject/fn-test-utils",
		// Delay: 0,
		Type: "sync",
		// Payload: TODO,
		Priority:    new(int32), // TODO this is crucial, apparently
		Timeout:     30,
		IdleTimeout: 30,
		Memory:      256,
		CreatedAt:   common.DateTime(time.Now()),
		URL:         "http://localhost:8080/r/myapp/thisisatest",
		Method:      "GET",
	}
	c2 := *call
	c3 := *call
	c2.CreatedAt = common.DateTime(time.Now().Add(100 * time.Second))
	c2.ID = id.New().String()
	c3.CreatedAt = common.DateTime(time.Now().Add(200 * time.Second))
	c3.ID = id.New().String()

	encodedC1ID := base64.RawURLEncoding.EncodeToString([]byte(call.ID))
	encodedC2ID := base64.RawURLEncoding.EncodeToString([]byte(c2.ID))
	encodedC3ID := base64.RawURLEncoding.EncodeToString([]byte(c3.ID))

	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMockInit(
		[]*models.Fn{fn},
	)
	fnl := logs.NewMock([]*models.Call{call, &c2, &c3})
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
		{"/v2/fns//calls", "", http.StatusBadRequest, models.ErrFnsMissingID, 0, ""},
		{"/v2/fns/nodawg/calls", "", http.StatusNotFound, models.ErrFnsNotFound, 0, ""},
		{"/v2/fns/fn_id/calls", "", http.StatusOK, nil, 3, ""},
		{"/v2/fns/fn_id/calls?per_page=1", "", http.StatusOK, nil, 1, encodedC3ID},
		{"/v2/fns/fn_id/calls?per_page=1&cursor=" + encodedC3ID, "", http.StatusOK, nil, 1, encodedC2ID},
		{"/v2/fns/fn_id/calls?per_page=1&cursor=" + encodedC2ID, "", http.StatusOK, nil, 1, encodedC1ID},
		{"/v2/fns/fn_id/calls?per_page=100&cursor=" + encodedC2ID, "", http.StatusOK, nil, 1, ""}, // cursor is empty if per_page > len(results)
		{"/v2/fns/fn_id/calls?per_page=1&cursor=" + encodedC1ID, "", http.StatusOK, nil, 0, ""},   // cursor could point to empty page
		{"/v2/fns/fn_id/calls?" + rangeTest, "", http.StatusOK, nil, 1, ""},
		{"/v2/fns/fn_id/calls?from_time=xyz", "", http.StatusBadRequest, models.ErrInvalidFromTime, 0, ""},
		{"/v2/fns/fn_id/calls?to_time=xyz", "", http.StatusBadRequest, models.ErrInvalidToTime, 0, ""},

		// // TODO path isn't url safe w/ '/', so this is weird. hack in for tests
		// {"/v2/fns/fn_id/calls?path=test2", "", http.StatusOK, nil, 1, ""},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if resp.Message == "" || !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Errorf("Test %d: Expected error message to have `%s`, got: `%s`",
					i, test.expectedError.Error(), resp.Message)
			}
		} else {
			// normal path

			var resp models.CallList
			err := json.NewDecoder(rec.Body).Decode(&resp)
			if err != nil {
				t.Errorf("Test %d: Expected response body to be a valid json object. err: %v", i, err)
			}
			if len(resp.Items) != test.expectedLen {
				t.Fatalf("Test %d: Expected calls length to be %d, but got %d", i, test.expectedLen, len(resp.Items))
			}
			if resp.NextCursor != test.nextCursor {
				t.Errorf("Test %d: Expected next_cursor to be %s, but got %s", i, test.nextCursor, resp.NextCursor)
			}
		}
	}
}
