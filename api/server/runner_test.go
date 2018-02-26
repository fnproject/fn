package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

func testRunner(t *testing.T, args ...interface{}) (agent.Agent, context.CancelFunc) {
	ds := datastore.NewMock()
	var mq models.MessageQueue = &mqs.Mock{}
	for _, a := range args {
		switch arg := a.(type) {
		case models.Datastore:
			ds = arg
		case models.MessageQueue:
			mq = arg
		}
	}
	r := agent.New(agent.NewDirectDataAccess(ds, ds, mq))
	return r, func() { r.Close() }
}

func TestRouteRunnerGet(t *testing.T) {
	buf := setLogBuffer()
	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		}, nil, nil,
	)

	rnr, cancel := testRunner(t, ds)
	defer cancel()
	logDB := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, logDB, rnr, ServerTypeFull)

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
			t.Errorf("Test %d: Expected status code for path %s to be %d but was %d",
				i, test.path, test.expectedCode, rec.Code)
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

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		}, nil, nil,
	)

	rnr, cancel := testRunner(t, ds)
	defer cancel()

	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

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
			t.Errorf("Test %d: Expected status code for path %s to be %d but was %d",
				i, test.path, test.expectedCode, rec.Code)
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
	isFailure := false
	// Log once after we are done, flow of events are important (hot/cold containers, idle timeout, etc.)
	// for figuring out why things failed.
	defer func() {
		if isFailure {
			t.Log(buf.String())
		}
	}()

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		},
		[]*models.Route{
			{Path: "/", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/myhot", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Format: "http", Memory: 128, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/myhotjason", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Format: "json", Memory: 128, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/myroute", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/myerror", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, Timeout: 30, IdleTimeout: 30, Headers: map[string][]string{"X-Function": {"Test"}}},
			{Path: "/mydne", AppName: "myapp", Image: "fnproject/imagethatdoesnotexist", Type: "sync", Memory: 128, Timeout: 30, IdleTimeout: 30},
			{Path: "/mydnehot", AppName: "myapp", Image: "fnproject/imagethatdoesnotexist", Type: "sync", Format: "http", Memory: 128, Timeout: 30, IdleTimeout: 30},
			{Path: "/mydneregistry", AppName: "myapp", Image: "localhost:5000/fnproject/imagethatdoesnotexist", Type: "sync", Format: "http", Memory: 128, Timeout: 30, IdleTimeout: 30},
			{Path: "/myoom", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 8, Timeout: 30, IdleTimeout: 30},
		}, nil,
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	fnl := logs.NewMock()

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	expHeaders := map[string][]string{"X-Function": {"Test"}}

	crasher := `{"sleepTime": 0, "isDebug": true, "isCrash": true}`          // crash container
	oomer := `{"sleepTime": 0, "isDebug": true, "allocateMemory": 12000000}` // ask for 12MB
	badHttp := `{"sleepTime": 0, "isDebug": true, "responseCode": -1}`       // http status of -1 (invalid http)
	badHot := `{"invalidResponse": true, "isDebug": true}`                   // write a not json/http as output
	ok := `{"sleepTime": 0, "isDebug": true}`                                // good response / ok

	for i, test := range []struct {
		path              string
		body              string
		method            string
		expectedCode      int
		expectedHeaders   map[string][]string
		expectedErrSubStr string
	}{
		{"/r/myapp/", ok, "GET", http.StatusOK, expHeaders, ""},

		{"/r/myapp/myhot", badHttp, "GET", http.StatusBadGateway, expHeaders, "invalid http response"},
		// hot container now back to normal, we should get OK
		{"/r/myapp/myhot", ok, "GET", http.StatusOK, expHeaders, ""},

		{"/r/myapp/myhot", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid http response"},
		{"/r/myapp/myhotjason", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid json response"},

		{"/r/myapp/myroute", ok, "GET", http.StatusOK, expHeaders, ""},
		{"/r/myapp/myerror", crasher, "GET", http.StatusBadGateway, expHeaders, "container exit code 2"},
		{"/r/myapp/mydne", ``, "GET", http.StatusNotFound, nil, "pull access denied"},
		{"/r/myapp/mydnehot", ``, "GET", http.StatusNotFound, nil, "pull access denied"},
		// hit a registry that doesn't exist, make sure the real error body gets plumbed out
		{"/r/myapp/mydneregistry", ``, "GET", http.StatusInternalServerError, nil, "connection refused"},

		{"/r/myapp/myoom", oomer, "GET", http.StatusBadGateway, nil, "container out of memory"},
	} {
		body := strings.NewReader(test.body)
		_, rec := routerRequest(t, srv.Router, test.method, test.path, body)
		respBytes, _ := ioutil.ReadAll(rec.Body)
		respBody := string(respBytes)
		maxLog := len(respBody)
		if maxLog > 1024 {
			maxLog = 1024
		}

		if rec.Code != test.expectedCode {
			isFailure = true
			t.Errorf("Test %d: Expected status code to be %d but was %d. body: %s",
				i, test.expectedCode, rec.Code, respBody[:maxLog])
		}

		if test.expectedErrSubStr != "" && !strings.Contains(respBody, test.expectedErrSubStr) {
			isFailure = true
			t.Errorf("Test %d: Expected response to include %s but got body: %s",
				i, test.expectedErrSubStr, respBody[:maxLog])

		}

		if test.expectedHeaders != nil {
			for name, header := range test.expectedHeaders {
				if header[0] != rec.Header().Get(name) {
					isFailure = true
					t.Errorf("Test %d: Expected header `%s` to be %s but was %s",
						i, name, header[0], rec.Header().Get(name))
				}
			}
		}
	}
}

// implement models.MQ and models.APIError
type errorMQ struct {
	error
	code int
}

func (mock *errorMQ) Push(context.Context, *models.Call) (*models.Call, error) { return nil, mock }
func (mock *errorMQ) Reserve(context.Context) (*models.Call, error)            { return nil, mock }
func (mock *errorMQ) Delete(context.Context, *models.Call) error               { return mock }
func (mock *errorMQ) Code() int                                                { return mock.code }

func TestFailedEnqueue(t *testing.T) {
	buf := setLogBuffer()
	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		},
		[]*models.Route{
			{Path: "/dummy", AppName: "myapp", Image: "dummy/dummy", Type: "async", Memory: 128, Timeout: 30, IdleTimeout: 30},
		}, nil,
	)
	err := errors.New("Unable to push task to queue")
	mq := &errorMQ{err, http.StatusInternalServerError}
	fnl := logs.NewMock()
	rnr, cancelrnr := testRunner(t, ds, mq)
	defer cancelrnr()

	srv := testServer(ds, mq, fnl, rnr, ServerTypeFull)
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
	buf := setLogBuffer()

	models.RouteMaxMemory = uint64(1024 * 1024 * 1024) // 1024 TB
	hugeMem := uint64(models.RouteMaxMemory - 1)

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		},
		[]*models.Route{
			{Path: "/cold", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, Timeout: 4, IdleTimeout: 30},
			{Path: "/hot", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Format: "http", Memory: 128, Timeout: 4, IdleTimeout: 30},
			{Path: "/hot-json", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Format: "json", Memory: 128, Timeout: 4, IdleTimeout: 30},
			{Path: "/bigmem-cold", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Memory: hugeMem, Timeout: 1, IdleTimeout: 30},
			{Path: "/bigmem-hot", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Format: "http", Memory: hugeMem, Timeout: 1, IdleTimeout: 30},
		}, nil,
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/r/myapp/cold", `{"sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
		{"/r/myapp/cold", `{"sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
		{"/r/myapp/hot", `{"sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
		{"/r/myapp/hot", `{"sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
		{"/r/myapp/hot-json", `{"sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
		{"/r/myapp/hot-json", `{"sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
		{"/r/myapp/bigmem-cold", `{"sleepTime": 0, "isDebug": true}`, "POST", http.StatusServiceUnavailable, map[string][]string{"Retry-After": {"15"}}},
		{"/r/myapp/bigmem-hot", `{"sleepTime": 0, "isDebug": true}`, "POST", http.StatusServiceUnavailable, map[string][]string{"Retry-After": {"15"}}},
	} {
		body := strings.NewReader(test.body)
		_, rec := routerRequest(t, srv.Router, test.method, test.path, body)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Errorf("Test %d: Expected status code to be %d but was %d body: %#v",
				i, test.expectedCode, rec.Code, rec.Body.String())
		}

		if test.expectedHeaders == nil {
			continue
		}
		for name, header := range test.expectedHeaders {
			if header[0] != rec.Header().Get(name) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected header `%s` to be %s but was %s body: %#v",
					i, name, header[0], rec.Header().Get(name), rec.Body.String())
			}
		}
	}
}

// Minimal test that checks the possibility of invoking concurrent hot sync functions.
func TestRouteRunnerMinimalConcurrentHotSync(t *testing.T) {
	buf := setLogBuffer()

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: "myapp", Config: models.Config{}},
		},
		[]*models.Route{
			{Path: "/hot", AppName: "myapp", Image: "fnproject/fn-test-utils", Type: "sync", Format: "http", Memory: 128, Timeout: 30, IdleTimeout: 5},
		}, nil,
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	fnl := logs.NewMock()
	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/r/myapp/hot", `{"sleepTime": 100, "isDebug": true}`, "POST", http.StatusOK, nil},
	} {
		errs := make(chan error)
		numCalls := 4
		for k := 0; k < numCalls; k++ {
			go func() {
				body := strings.NewReader(test.body)
				_, rec := routerRequest(t, srv.Router, test.method, test.path, body)

				if rec.Code != test.expectedCode {
					t.Log(buf.String())
					errs <- fmt.Errorf("Test %d: Expected status code to be %d but was %d body: %#v",
						i, test.expectedCode, rec.Code, rec.Body.String())
					return
				}

				if test.expectedHeaders == nil {
					errs <- nil
					return
				}
				for name, header := range test.expectedHeaders {
					if header[0] != rec.Header().Get(name) {
						t.Log(buf.String())
						errs <- fmt.Errorf("Test %d: Expected header `%s` to be %s but was %s body: %#v",
							i, name, header[0], rec.Header().Get(name), rec.Body.String())
						return
					}
				}
				errs <- nil
			}()
		}
		for k := 0; k < numCalls; k++ {
			err := <-errs
			if err != nil {
				t.Errorf("%v", err)
			}
		}
	}
}

//func TestMatchRoute(t *testing.T) {
//buf := setLogBuffer()
//for i, test := range []struct {
//baseRoute      string
//route          string
//expectedParams []Param
//}{
//{"/myroute/", `/myroute/`, nil},
//{"/myroute/:mybigparam", `/myroute/1`, []Param{{"mybigparam", "1"}}},
//{"/:param/*test", `/1/2`, []Param{{"param", "1"}, {"test", "/2"}}},
//} {
//if params, match := matchRoute(test.baseRoute, test.route); match {
//if test.expectedParams != nil {
//for j, param := range test.expectedParams {
//if params[j].Key != param.Key || params[j].Value != param.Value {
//t.Log(buf.String())
//t.Errorf("Test %d: expected param %d, key = %s, value = %s", i, j, param.Key, param.Value)
//}
//}
//}
//} else {
//t.Log(buf.String())
//t.Errorf("Test %d: %s should match %s", i, test.route, test.baseRoute)
//}
//}
//}
