package server

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"errors"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/fnproject/fn/api/runner"
)

func testRunner(t *testing.T) (*runner.Runner, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ds := datastore.NewMock()
	fnl := logs.NewMock()
	r, err := runner.New(ctx, runner.NewFuncLogger(fnl), ds)
	if err != nil {
		t.Fatal("Test: failed to create new runner")
	}
	return r, cancel
}

func TestRouteRunnerGet(t *testing.T) {
	buf := setLogBuffer()
	rnr, cancel := testRunner(t)
	defer cancel()
	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		}, nil, nil, nil,
	)
	logDB := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, DefaultEnqueue)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/route", "", http.StatusNotFound, nil},
		{"/r/app/route", "", http.StatusNotFound, models.ErrAppsNotFound},
		{"/r/myapp/route", "", http.StatusNotFound, models.ErrRoutesNotFound},
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
				t.Errorf("Test %d: Expected error message to have `%s`, but got `%s`",
					i, test.expectedError.Error(), resp.Error.Message)
			}
		}
	}
}

func TestRouteRunnerPost(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancel := testRunner(t)
	defer cancel()

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		}, nil, nil, nil,
	)
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, DefaultEnqueue)

	for i, test := range []struct {
		path          string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/route", `{ "payload": "" }`, http.StatusNotFound, nil},
		{"/r/app/route", `{ "payload": "" }`, http.StatusNotFound, models.ErrAppsNotFound},
		{"/r/myapp/route", `{ "payload": "" }`, http.StatusNotFound, models.ErrRoutesNotFound},
	} {
		body := bytes.NewBuffer([]byte(test.body))
		_, rec := routerRequest(t, srv.Router, "POST", test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)
			respMsg := resp.Error.Message
			expMsg := test.expectedError.Error()
			if respMsg != expMsg && !strings.Contains(respMsg, expMsg) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestRouteRunnerExecution(t *testing.T) {
	buf := setLogBuffer()

	rnr, cancelrnr := testRunner(t)
	defer cancelrnr()

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		},
		[]*models.Route{
			{Path: "/", AppName: "myapp", Image: "funcy/hello", Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/myroute", AppName: "myapp", Image: "funcy/hello", Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/myerror", AppName: "myapp", Image: "funcy/error", Headers: map[string][]string{"X-Function": {"Test"}}},
		}, nil, nil,
	)

	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, DefaultEnqueue)

	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/r/myapp/", ``, "GET", http.StatusOK, map[string][]string{"X-Function": {"Test"}}},
		{"/r/myapp/myroute", ``, "GET", http.StatusOK, map[string][]string{"X-Function": {"Test"}}},
		{"/r/myapp/myerror", ``, "GET", http.StatusInternalServerError, map[string][]string{"X-Function": {"Test"}}},

		// Added same tests again to check if time is reduced by the auth cache
		{"/r/myapp/", ``, "GET", http.StatusOK, map[string][]string{"X-Function": {"Test"}}},
		{"/r/myapp/myroute", ``, "GET", http.StatusOK, map[string][]string{"X-Function": {"Test"}}},
		{"/r/myapp/myerror", ``, "GET", http.StatusInternalServerError, map[string][]string{"X-Function": {"Test"}}},
	} {
		body := strings.NewReader(test.body)
		_, rec := routerRequest(t, srv.Router, test.method, test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedHeaders == nil {
			continue
		}
		for name, header := range test.expectedHeaders {
			if header[0] != rec.Header().Get(name) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected header `%s` to be %s but was %s",
					i, name, header[0], rec.Header().Get(name))
			}
		}
	}
}

func TestFailedEnqueue(t *testing.T) {
	buf := setLogBuffer()
	rnr, cancelrnr := testRunner(t)
	defer cancelrnr()

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		},
		[]*models.Route{
			{Path: "/dummy", AppName: "myapp", Image: "dummy/dummy", Type: "async"},
		}, nil, nil,
	)
	fnl := logs.NewMock()

	enqueue := func(ctx context.Context, mq models.MessageQueue, task *models.Task) (*models.Task, error) {
		return nil, errors.New("Unable to push task to queue")
	}

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, enqueue)
	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/r/myapp/dummy", ``, "POST", http.StatusInternalServerError, nil},
	} {
		body := strings.NewReader(test.body)
		_, rec := routerRequest(t, srv.Router, test.method, test.path, body)
		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}
	}
}

func TestRouteRunnerTimeout(t *testing.T) {
	t.Skip("doesn't work on old Ubuntu")
	buf := setLogBuffer()

	rnr, cancelrnr := testRunner(t)
	defer cancelrnr()

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		},
		[]*models.Route{
			{Path: "/sleeper", AppName: "myapp", Image: "funcy/sleeper", Timeout: 1},
		}, nil, nil,
	)
	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, DefaultEnqueue)

	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/r/myapp/sleeper", `{"sleep": 0}`, "POST", http.StatusOK, nil},
		{"/r/myapp/sleeper", `{"sleep": 2}`, "POST", http.StatusGatewayTimeout, nil},
	} {
		body := strings.NewReader(test.body)
		_, rec := routerRequest(t, srv.Router, test.method, test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d",
				i, test.expectedCode, rec.Code)
		}

		if test.expectedHeaders == nil {
			continue
		}
		for name, header := range test.expectedHeaders {
			if header[0] != rec.Header().Get(name) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected header `%s` to be %s but was %s",
					i, name, header[0], rec.Header().Get(name))
			}
		}
	}
}

func TestMatchRoute(t *testing.T) {
	buf := setLogBuffer()
	for i, test := range []struct {
		baseRoute      string
		route          string
		expectedParams []Param
	}{
		{"/myroute/", `/myroute/`, nil},
		{"/myroute/:mybigparam", `/myroute/1`, []Param{{"mybigparam", "1"}}},
		{"/:param/*test", `/1/2`, []Param{{"param", "1"}, {"test", "/2"}}},
	} {
		if params, match := matchRoute(test.baseRoute, test.route); match {
			if test.expectedParams != nil {
				for j, param := range test.expectedParams {
					if params[j].Key != param.Key || params[j].Value != param.Value {
						t.Log(buf.String())
						t.Errorf("Test %d: expected param %d, key = %s, value = %s", i, j, param.Key, param.Value)
					}
				}
			}
		} else {
			t.Log(buf.String())
			t.Errorf("Test %d: %s should match %s", i, test.route, test.baseRoute)
		}
	}
}
