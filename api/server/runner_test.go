package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

func envTweaker(name, value string) func() {
	bck, ok := os.LookupEnv(name)

	err := os.Setenv(name, value)
	if err != nil {
		panic(err.Error())
	}

	return func() {
		var err error
		if !ok {
			err = os.Unsetenv(name)
		} else {
			err = os.Setenv(name, bck)
		}
		if err != nil {
			panic(err.Error())
		}
	}
}

func testRunner(_ *testing.T, args ...interface{}) (agent.Agent, context.CancelFunc) {
	ds := datastore.NewMock()
	ls := logs.NewMock()
	var mq models.MessageQueue = &mqs.Mock{}
	for _, a := range args {
		switch arg := a.(type) {
		case models.Datastore:
			ds = arg
		case models.MessageQueue:
			mq = arg
		case models.LogStore:
			ls = arg
		}
	}
	r := agent.New(agent.NewDirectDataAccess(ds, ls, mq))
	return r, func() { r.Close() }
}

func TestRouteRunnerGet(t *testing.T) {
	buf := setLogBuffer()
	app := &models.App{Name: "myapp", Config: models.Config{}}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
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
			resp := getV1ErrorResponse(t, rec)

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

	app := &models.App{Name: "myapp", Config: models.Config{}}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
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
			resp := getV1ErrorResponse(t, rec)
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

func TestRouteRunnerExecEmptyBody(t *testing.T) {
	buf := setLogBuffer()
	isFailure := false

	defer func() {
		if isFailure {
			t.Log(buf.String())
		}
	}()

	rCfg := map[string]string{"ENABLE_HEADER": "yes", "ENABLE_FOOTER": "yes"} // enable container start/end header/footer
	rHdr := map[string][]string{"X-Function": {"Test"}}
	rImg := "fnproject/fn-test-utils"

	app := &models.App{Name: "soup"}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{Path: "/cold", AppID: app.ID, Image: rImg, Type: "sync", Memory: 64, Timeout: 10, IdleTimeout: 20, Headers: rHdr, Config: rCfg},
			{Path: "/hothttp", AppID: app.ID, Image: rImg, Type: "sync", Format: "http", Memory: 64, Timeout: 10, IdleTimeout: 20, Headers: rHdr, Config: rCfg},
			{Path: "/hotjson", AppID: app.ID, Image: rImg, Type: "sync", Format: "json", Memory: 64, Timeout: 10, IdleTimeout: 20, Headers: rHdr, Config: rCfg},
		},
	)
	ls := logs.NewMock()

	rnr, cancelrnr := testRunner(t, ds, ls)
	defer cancelrnr()

	srv := testServer(ds, &mqs.Mock{}, ls, rnr, ServerTypeFull)

	expHeaders := map[string][]string{"X-Function": {"Test"}}
	emptyBody := `{"echoContent": "_TRX_ID_", "isDebug": true, "isEmptyBody": true}`

	// Test hot cases twice to rule out hot-containers corrupting next request.
	testCases := []struct {
		path string
	}{
		{"/r/soup/cold/"},
		{"/r/soup/hothttp"},
		{"/r/soup/hothttp"},
		{"/r/soup/hotjson"},
		{"/r/soup/hotjson"},
	}

	for i, test := range testCases {
		trx := fmt.Sprintf("_trx_%d_", i)
		body := strings.NewReader(strings.Replace(emptyBody, "_TRX_ID_", trx, 1))
		_, rec := routerRequest(t, srv.Router, "GET", test.path, body)
		respBytes, _ := ioutil.ReadAll(rec.Body)
		respBody := string(respBytes)
		maxBody := len(respBody)
		if maxBody > 1024 {
			maxBody = 1024
		}

		if rec.Code != http.StatusOK {
			isFailure = true
			t.Errorf("Test %d: Expected status code to be %d but was %d. body: %s",
				i, http.StatusOK, rec.Code, respBody[:maxBody])
		} else if len(respBytes) != 0 {
			isFailure = true
			t.Errorf("Test %d: Expected empty body but got %d. body: %s",
				i, len(respBytes), respBody[:maxBody])
		}

		for name, header := range expHeaders {
			if header[0] != rec.Header().Get(name) {
				isFailure = true
				t.Errorf("Test %d: Expected header `%s` to be %s but was %s. body: %s",
					i, name, header[0], rec.Header().Get(name), respBody)
			}
		}
	}
}
func TestRouteRunnerExecution(t *testing.T) {
	buf := setLogBuffer()
	isFailure := false
	tweaker := envTweaker("FN_MAX_RESPONSE_SIZE", "2048")
	defer tweaker()

	// Log once after we are done, flow of events are important (hot/cold containers, idle timeout, etc.)
	// for figuring out why things failed.
	defer func() {
		if isFailure {
			t.Log(buf.String())
		}
	}()

	rCfg := map[string]string{"ENABLE_HEADER": "yes", "ENABLE_FOOTER": "yes"} // enable container start/end header/footer
	rHdr := map[string][]string{"X-Function": {"Test"}}
	rImg := "fnproject/fn-test-utils"
	rImgBs1 := "fnproject/imagethatdoesnotexist"
	rImgBs2 := "localhost:5050/fnproject/imagethatdoesnotexist"

	app := &models.App{Name: "myapp"}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{Path: "/", AppID: app.ID, Image: rImg, Type: "sync", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/myhot", AppID: app.ID, Image: rImg, Type: "sync", Format: "http", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/myhotjason", AppID: app.ID, Image: rImg, Type: "sync", Format: "json", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/myroute", AppID: app.ID, Image: rImg, Type: "sync", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/myerror", AppID: app.ID, Image: rImg, Type: "sync", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/mydne", AppID: app.ID, Image: rImgBs1, Type: "sync", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/mydnehot", AppID: app.ID, Image: rImgBs1, Type: "sync", Format: "http", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/mydneregistry", AppID: app.ID, Image: rImgBs2, Type: "sync", Format: "http", Memory: 64, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/myoom", AppID: app.ID, Image: rImg, Type: "sync", Memory: 8, Timeout: 30, IdleTimeout: 30, Headers: rHdr, Config: rCfg},
			{Path: "/mybigoutputcold", AppID: app.ID, Image: rImg, Type: "sync", Memory: 64, Timeout: 10, IdleTimeout: 20, Headers: rHdr, Config: rCfg},
			{Path: "/mybigoutputhttp", AppID: app.ID, Image: rImg, Type: "sync", Format: "http", Memory: 64, Timeout: 10, IdleTimeout: 20, Headers: rHdr, Config: rCfg},
			{Path: "/mybigoutputjson", AppID: app.ID, Image: rImg, Type: "sync", Format: "json", Memory: 64, Timeout: 10, IdleTimeout: 20, Headers: rHdr, Config: rCfg},
		},
	)
	ls := logs.NewMock()

	rnr, cancelrnr := testRunner(t, ds, ls)
	defer cancelrnr()

	srv := testServer(ds, &mqs.Mock{}, ls, rnr, ServerTypeFull)

	expHeaders := map[string][]string{"X-Function": {"Test"}, "Content-Type": {"application/json; charset=utf-8"}}
	expCTHeaders := map[string][]string{"X-Function": {"Test"}, "Content-Type": {"foo/bar"}}

	// Checking for EndOfLogs currently depends on scheduling of go-routines (in docker/containerd) that process stderr & stdout.
	// Therefore, not testing for EndOfLogs for hot containers (which has complex I/O processing) anymore.
	multiLogExpectCold := []string{"BeginOfLogs", "EndOfLogs"}
	multiLogExpectHot := []string{"BeginOfLogs" /*, "EndOfLogs" */}

	crasher := `{"echoContent": "_TRX_ID_", "isDebug": true, "isCrash": true}`                      // crash container
	oomer := `{"echoContent": "_TRX_ID_", "isDebug": true, "allocateMemory": 12000000}`             // ask for 12MB
	badHot := `{"echoContent": "_TRX_ID_", "invalidResponse": true, "isDebug": true}`               // write a not json/http as output
	ok := `{"echoContent": "_TRX_ID_", "isDebug": true}`                                            // good response / ok
	respTypeLie := `{"echoContent": "_TRX_ID_", "responseContentType": "foo/bar", "isDebug": true}` // Content-Type: foo/bar
	respTypeJason := `{"echoContent": "_TRX_ID_", "jasonContentType": "foo/bar", "isDebug": true}`  // Content-Type: foo/bar

	// sleep between logs and with debug enabled, fn-test-utils will log header/footer below:
	multiLog := `{"echoContent": "_TRX_ID_", "sleepTime": 1000, "isDebug": true}`
	bigoutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1000}` // 1000 trailers to exceed 2K
	smalloutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1}`  // 1 trailer < 2K

	testCases := []struct {
		path               string
		body               string
		method             string
		expectedCode       int
		expectedHeaders    map[string][]string
		expectedErrSubStr  string
		expectedLogsSubStr []string
	}{
		{"/r/myapp/", ok, "GET", http.StatusOK, expHeaders, "", nil},

		{"/r/myapp/myhot", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid http response", nil},
		// hot container now back to normal:
		{"/r/myapp/myhot", ok, "GET", http.StatusOK, expHeaders, "", nil},

		{"/r/myapp/myhotjason", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid json response", nil},
		// hot container now back to normal:
		{"/r/myapp/myhotjason", ok, "GET", http.StatusOK, expHeaders, "", nil},

		{"/r/myapp/myhot", respTypeLie, "GET", http.StatusOK, expCTHeaders, "", nil},
		{"/r/myapp/myhotjason", respTypeLie, "GET", http.StatusOK, expCTHeaders, "", nil},
		{"/r/myapp/myhotjason", respTypeJason, "GET", http.StatusOK, expCTHeaders, "", nil},

		{"/r/myapp/myroute", ok, "GET", http.StatusOK, expHeaders, "", nil},
		{"/r/myapp/myerror", crasher, "GET", http.StatusBadGateway, expHeaders, "container exit code 2", nil},
		{"/r/myapp/mydne", ``, "GET", http.StatusNotFound, nil, "pull access denied", nil},
		{"/r/myapp/mydnehot", ``, "GET", http.StatusNotFound, nil, "pull access denied", nil},
		// hit a registry that doesn't exist, make sure the real error body gets plumbed out
		{"/r/myapp/mydneregistry", ``, "GET", http.StatusInternalServerError, nil, "connection refused", nil},

		{"/r/myapp/myoom", oomer, "GET", http.StatusBadGateway, nil, "container out of memory", nil},
		{"/r/myapp/myhot", multiLog, "GET", http.StatusOK, nil, "", multiLogExpectHot},
		{"/r/myapp/", multiLog, "GET", http.StatusOK, nil, "", multiLogExpectCold},
		{"/r/myapp/mybigoutputjson", bigoutput, "GET", http.StatusBadGateway, nil, "function response too large", nil},
		{"/r/myapp/mybigoutputjson", smalloutput, "GET", http.StatusOK, nil, "", nil},
		{"/r/myapp/mybigoutputhttp", bigoutput, "GET", http.StatusBadGateway, nil, "", nil},
		{"/r/myapp/mybigoutputhttp", smalloutput, "GET", http.StatusOK, nil, "", nil},
		{"/r/myapp/mybigoutputcold", bigoutput, "GET", http.StatusBadGateway, nil, "", nil},
		{"/r/myapp/mybigoutputcold", smalloutput, "GET", http.StatusOK, nil, "", nil},
	}

	callIds := make([]string, len(testCases))

	for i, test := range testCases {
		trx := fmt.Sprintf("_trx_%d_", i)
		body := strings.NewReader(strings.Replace(test.body, "_TRX_ID_", trx, 1))
		_, rec := routerRequest(t, srv.Router, test.method, test.path, body)
		respBytes, _ := ioutil.ReadAll(rec.Body)
		respBody := string(respBytes)
		maxBody := len(respBody)
		if maxBody > 1024 {
			maxBody = 1024
		}

		callIds[i] = rec.Header().Get("Fn_call_id")

		if rec.Code != test.expectedCode {
			isFailure = true
			t.Errorf("Test %d: Expected status code to be %d but was %d. body: %s",
				i, test.expectedCode, rec.Code, respBody[:maxBody])
		}

		if rec.Code == http.StatusOK && !strings.Contains(respBody, trx) {
			isFailure = true
			t.Errorf("Test %d: Expected response to include %s but got body: %s",
				i, trx, respBody[:maxBody])

		}

		if test.expectedErrSubStr != "" && !strings.Contains(respBody, test.expectedErrSubStr) {
			isFailure = true
			t.Errorf("Test %d: Expected response to include %s but got body: %s",
				i, test.expectedErrSubStr, respBody[:maxBody])

		}

		if test.expectedHeaders != nil {
			for name, header := range test.expectedHeaders {
				if header[0] != rec.Header().Get(name) {
					isFailure = true
					t.Errorf("Test %d: Expected header `%s` to be %s but was %s. body: %s",
						i, name, header[0], rec.Header().Get(name), respBody)
				}
			}
		}
	}

	for i, test := range testCases {
		if test.expectedLogsSubStr != nil {
			if !checkLogs(t, i, ls, callIds[i], test.expectedLogsSubStr) {
				isFailure = true
			}
		}
	}
}

func getDockerId(respBytes []byte) (string, error) {

	var respJs map[string]interface{}
	var data map[string]interface{}

	err := json.Unmarshal(respBytes, &respJs)
	if err != nil {
		return "", err
	}

	data, ok := respJs["data"].(map[string]interface{})
	if !ok {
		return "", errors.New("unexpected json: data map")
	}

	id, ok := data["DockerId"].(string)
	if !ok {
		return "", errors.New("unexpected json: docker id string")
	}

	return id, nil
}

func checkLogs(t *testing.T, tnum int, ds models.LogStore, callID string, expected []string) bool {

	logReader, err := ds.GetLog(context.Background(), "myapp", callID)
	if err != nil {
		t.Errorf("Test %d: GetLog for call_id:%s returned err %s",
			tnum, callID, err.Error())
		return false
	}

	logBytes, err := ioutil.ReadAll(logReader)
	if err != nil {
		t.Errorf("Test %d: GetLog read IO call_id:%s returned err %s",
			tnum, callID, err.Error())
		return false
	}

	logBody := string(logBytes)
	maxLog := len(logBody)
	if maxLog > 1024 {
		maxLog = 1024
	}

	for _, match := range expected {
		if !strings.Contains(logBody, match) {
			t.Errorf("Test %d: GetLog read IO call_id:%s cannot find: %s in logs: %s",
				tnum, callID, match, logBody[:maxLog])
			return false
		}
	}

	return true
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
func (mock *errorMQ) Close() error                                             { return nil }
func TestFailedEnqueue(t *testing.T) {
	buf := setLogBuffer()
	app := &models.App{Name: "myapp", Config: models.Config{}}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{Path: "/dummy", Image: "dummy/dummy", Type: "async", Memory: 128, Timeout: 30, IdleTimeout: 30, AppID: app.ID},
		},
	)
	err := errors.New("Unable to push task to queue")
	mq := &errorMQ{err, http.StatusInternalServerError}
	fnl := logs.NewMock()
	rnr, cancelrnr := testRunner(t, ds, mq, fnl)
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
	isFailure := false

	// Log once after we are done, flow of events are important (hot/cold containers, idle timeout, etc.)
	// for figuring out why things failed.
	defer func() {
		if isFailure {
			t.Log(buf.String())
		}
	}()

	models.RouteMaxMemory = uint64(1024 * 1024 * 1024) // 1024 TB
	hugeMem := uint64(models.RouteMaxMemory - 1)

	app := &models.App{Name: "myapp", Config: models.Config{}}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{Path: "/cold", Image: "fnproject/fn-test-utils", Type: "sync", Memory: 128, Timeout: 4, IdleTimeout: 30, AppID: app.ID},
			{Path: "/hot", Image: "fnproject/fn-test-utils", Type: "sync", Format: "http", Memory: 128, Timeout: 4, IdleTimeout: 30, AppID: app.ID},
			{Path: "/hot-json", Image: "fnproject/fn-test-utils", Type: "sync", Format: "json", Memory: 128, Timeout: 4, IdleTimeout: 30, AppID: app.ID},
			{Path: "/bigmem-cold", Image: "fnproject/fn-test-utils", Type: "sync", Memory: hugeMem, Timeout: 1, IdleTimeout: 30, AppID: app.ID},
			{Path: "/bigmem-hot", Image: "fnproject/fn-test-utils", Type: "sync", Format: "http", Memory: hugeMem, Timeout: 1, IdleTimeout: 30, AppID: app.ID},
		},
	)

	fnl := logs.NewMock()
	rnr, cancelrnr := testRunner(t, ds, fnl)
	defer cancelrnr()

	srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/r/myapp/cold", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
		{"/r/myapp/cold", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
		{"/r/myapp/hot", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
		{"/r/myapp/hot", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
		{"/r/myapp/hot-json", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
		{"/r/myapp/hot-json", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
		{"/r/myapp/bigmem-cold", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusServiceUnavailable, map[string][]string{"Retry-After": {"15"}}},
		{"/r/myapp/bigmem-hot", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusServiceUnavailable, map[string][]string{"Retry-After": {"15"}}},
	} {
		trx := fmt.Sprintf("_trx_%d_", i)
		body := strings.NewReader(strings.Replace(test.body, "_TRX_ID_", trx, 1))
		_, rec := routerRequest(t, srv.Router, test.method, test.path, body)
		respBytes, _ := ioutil.ReadAll(rec.Body)
		respBody := string(respBytes)
		maxBody := len(respBody)
		if maxBody > 1024 {
			maxBody = 1024
		}

		if rec.Code != test.expectedCode {
			isFailure = true
			t.Errorf("Test %d: Expected status code to be %d but was %d body: %#v",
				i, test.expectedCode, rec.Code, respBody[:maxBody])
		}

		if rec.Code == http.StatusOK && !strings.Contains(respBody, trx) {
			isFailure = true
			t.Errorf("Test %d: Expected response to include %s but got body: %s",
				i, trx, respBody[:maxBody])

		}

		if test.expectedHeaders != nil {
			for name, header := range test.expectedHeaders {
				if header[0] != rec.Header().Get(name) {
					isFailure = true
					t.Errorf("Test %d: Expected header `%s` to be %s but was %s body: %#v",
						i, name, header[0], rec.Header().Get(name), respBody[:maxBody])
				}
			}
		}
	}
}

// Minimal test that checks the possibility of invoking concurrent hot sync functions.
func TestRouteRunnerMinimalConcurrentHotSync(t *testing.T) {
	buf := setLogBuffer()

	app := &models.App{Name: "myapp", Config: models.Config{}}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{Path: "/hot", AppID: app.ID, Image: "fnproject/fn-test-utils", Type: "sync", Format: "http", Memory: 128, Timeout: 30, IdleTimeout: 5},
		},
	)

	fnl := logs.NewMock()
	rnr, cancelrnr := testRunner(t, ds, fnl)
	defer cancelrnr()

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
