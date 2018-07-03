package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

func TestTriggerRunnerGet(t *testing.T) {
	buf := setLogBuffer()
	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
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
		{"/t/app/route", "", http.StatusNotFound, models.ErrAppsNotFound},
		{"/t/myapp/route", "", http.StatusNotFound, models.ErrTriggerNotFound},
	} {
		_, rec := routerRequest(t, srv.Router, "GET", test.path, nil)

		if rec.Code != test.expectedCode {
			t.Log(buf.String())
			t.Fatalf("Test %d: Expected status code for path %s to be %d but was %d",
				i, test.path, test.expectedCode, rec.Code)
		}

		if test.expectedError != nil {
			resp := getErrorResponse(t, rec)

			if !strings.Contains(resp.Message, test.expectedError.Error()) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`, but got `%s`",
					i, test.expectedError.Error(), resp.Message)
			}
		}
	}
}

func TestTriggerRunnerPost(t *testing.T) {
	buf := setLogBuffer()

	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
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
		{"/t/app/route", `{ "payload": "" }`, http.StatusNotFound, models.ErrAppsNotFound},
		{"/t/myapp/route", `{ "payload": "" }`, http.StatusNotFound, models.ErrTriggerNotFound},
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
			respMsg := resp.Message
			expMsg := test.expectedError.Error()
			if respMsg != expMsg && !strings.Contains(respMsg, expMsg) {
				t.Log(buf.String())
				t.Errorf("Test %d: Expected error message to have `%s`",
					i, test.expectedError.Error())
			}
		}
	}
}

func TestTriggerRunnerExecEmptyBody(t *testing.T) {
	buf := setLogBuffer()
	isFailure := false

	defer func() {
		if isFailure {
			t.Log(buf.String())
		}
	}()

	rCfg := map[string]string{"ENABLE_HEADER": "yes", "ENABLE_FOOTER": "yes"} // enable container start/end header/footer
	rImg := "fnproject/fn-test-utils"

	app := &models.App{ID: "app_id", Name: "soup"}

	f1 := &models.Fn{ID: "cold", Name: "cold", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 10, IdleTimeout: 20}, Config: rCfg}
	f2 := &models.Fn{ID: "hothttp", Name: "hothttp", AppID: app.ID, Image: rImg, Format: "http", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 10, IdleTimeout: 20}, Config: rCfg}
	f3 := &models.Fn{ID: "hotjson", Name: "hotjson", AppID: app.ID, Image: rImg, Format: "json", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 10, IdleTimeout: 20}, Config: rCfg}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{f1, f2, f3},
		[]*models.Trigger{
			{ID: "t1", Name: "t1", AppID: app.ID, FnID: f1.ID, Type: "http", Source: "/cold"},
			{ID: "t2", Name: "t2", AppID: app.ID, FnID: f1.ID, Type: "http", Source: "/hothttp"},
			{ID: "t3", Name: "t3", AppID: app.ID, FnID: f1.ID, Type: "http", Source: "/hotjson"},
		},
	)
	ls := logs.NewMock()

	rnr, cancelrnr := testRunner(t, ds, ls)
	defer cancelrnr()

	srv := testServer(ds, &mqs.Mock{}, ls, rnr, ServerTypeFull)

	emptyBody := `{"echoContent": "_TRX_ID_", "isDebug": true, "isEmptyBody": true}`

	// Test hot cases twice to rule out hot-containers corrupting next request.
	testCases := []struct {
		path string
	}{
		{"/t/soup/cold/"},
		{"/t/soup/hothttp"},
		{"/t/soup/hothttp"},
		{"/t/soup/hotjson"},
		{"/t/soup/hotjson"},
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

	}
}

func TestTriggerRunnerExecution(t *testing.T) {
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
	rImg := "fnproject/fn-test-utils"
	rImgBs1 := "fnproject/imagethatdoesnotexist"
	rImgBs2 := "localhost:5050/fnproject/imagethatdoesnotexist"

	app := &models.App{ID: "app_id", Name: "myapp"}

	defaultDneFn := &models.Fn{ID: "default_dne_fn_id", Name: "default_dne_fn", AppID: app.ID, Image: rImgBs1, Format: "", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	defaultFn := &models.Fn{ID: "default_fn_id", Name: "default_fn", AppID: app.ID, Image: rImg, Format: "", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	httpFn := &models.Fn{ID: "http_fn_id", Name: "http_fn", AppID: app.ID, Image: rImg, Format: "http", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	httpDneFn := &models.Fn{ID: "http_dne_fn_id", Name: "http_dne_fn", AppID: app.ID, Image: rImgBs1, Format: "http", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	httpDneRegistryFn := &models.Fn{ID: "http_dnereg_fn_id", Name: "http_dnereg_fn", AppID: app.ID, Image: rImgBs2, Format: "http", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	jsonFn := &models.Fn{ID: "json_fn_id", Name: "json_fn", AppID: app.ID, Image: rImg, Format: "json", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	oomFn := &models.Fn{ID: "http_fn_id", Name: "http_fn", AppID: app.ID, Image: rImg, Format: "http", ResourceConfig: models.ResourceConfig{Memory: 8, Timeout: 30, IdleTimeout: 30}, Config: rCfg}

	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{defaultFn, defaultDneFn, httpDneRegistryFn, oomFn, httpFn, jsonFn},
		[]*models.Trigger{
			{ID: "1", Name: "1", Source: "/", Type: "http", AppID: app.ID, FnID: defaultFn.ID},
			{ID: "2", Name: "2", Source: "/myhot", Type: "http", AppID: app.ID, FnID: httpFn.ID},
			{ID: "3", Name: "3", Source: "/myhotjason", Type: "http", AppID: app.ID, FnID: jsonFn.ID},
			{ID: "4", Name: "4", Source: "/myroute", Type: "http", AppID: app.ID, FnID: defaultFn.ID},
			{ID: "5", Name: "5", Source: "/myerror", Type: "http", AppID: app.ID, FnID: defaultFn.ID},
			{ID: "6", Name: "6", Source: "/mydne", Type: "http", AppID: app.ID, FnID: defaultDneFn.ID},
			{ID: "7", Name: "7", Source: "/mydnehot", Type: "http", AppID: app.ID, FnID: httpDneFn.ID},
			{ID: "8", Name: "8", Source: "/mydneregistry", Type: "http", AppID: app.ID, FnID: httpDneRegistryFn.ID},
			{ID: "9", Name: "9", Source: "/myoom", Type: "http", AppID: app.ID, FnID: oomFn.ID},
			{ID: "10", Name: "10", Source: "/mybigoutputcold", Type: "http", AppID: app.ID, FnID: defaultFn.ID},
			{ID: "11", Name: "11", Source: "/mybigoutputhttp", Type: "http", AppID: app.ID, FnID: httpFn.ID},
			{ID: "12", Name: "12", Source: "/mybigoutputjson", Type: "http", AppID: app.ID, FnID: jsonFn.ID},
		},
	)
	ls := logs.NewMock()

	rnr, cancelrnr := testRunner(t, ds, ls)
	defer cancelrnr()

	srv := testServer(ds, &mqs.Mock{}, ls, rnr, ServerTypeFull)

	//expHeaders := map[string][]string{"Content-Type": {"application/json; charset=utf-8"}}
	//expCTHeaders := map[string][]string{"Content-Type": {"foo/bar"}}
	//
	//// Checking for EndOfLogs currently depends on scheduling of go-routines (in docker/containerd) that process stderr & stdout.
	//// Therefore, not testing for EndOfLogs for hot containers (which has complex I/O processing) anymore.
	//multiLogExpectCold := []string{"BeginOfLogs", "EndOfLogs"}
	//multiLogExpectHot := []string{"BeginOfLogs" /*, "EndOfLogs" */}
	//
	//crasher := `{"echoContent": "_TRX_ID_", "isDebug": true, "isCrash": true}`                      // crash container
	//oomer := `{"echoContent": "_TRX_ID_", "isDebug": true, "allocateMemory": 12000000}`             // ask for 12MB
	//badHot := `{"echoContent": "_TRX_ID_", "invalidResponse": true, "isDebug": true}`               // write a not json/http as output
	//ok := `{"echoContent": "_TRX_ID_", "isDebug": true}`                                            // good response / ok
	//respTypeLie := `{"echoContent": "_TRX_ID_", "responseContentType": "foo/bar", "isDebug": true}` // Content-Type: foo/bar
	//respTypeJason := `{"echoContent": "_TRX_ID_", "jasonContentType": "foo/bar", "isDebug": true}`  // Content-Type: foo/bar
	//
	//// sleep between logs and with debug enabled, fn-test-utils will log header/footer below:
	//multiLog := `{"echoContent": "_TRX_ID_", "sleepTime": 1000, "isDebug": true}`
	//bigoutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1000}` // 1000 trailers to exceed 2K
	//smalloutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1}`  // 1 trailer < 2K

	testCases := []struct {
		path               string
		body               string
		method             string
		expectedCode       int
		expectedHeaders    map[string][]string
		expectedErrSubStr  string
		expectedLogsSubStr []string
	}{
		//{"/t/myapp/", ok, "GET", http.StatusOK, expHeaders, "", nil},
		//
		//{"/t/myapp/myhot", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid http response", nil},
		//// hot container now back to normal:
		//{"/t/myapp/myhot", ok, "GET", http.StatusOK, expHeaders, "", nil},
		//
		//{"/t/myapp/myhotjason", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid json response", nil},
		//// hot container now back to normal:
		//{"/t/myapp/myhotjason", ok, "GET", http.StatusOK, expHeaders, "", nil},
		//
		//{"/t/myapp/myhot", respTypeLie, "GET", http.StatusOK, expCTHeaders, "", nil},
		//{"/t/myapp/myhotjason", respTypeLie, "GET", http.StatusOK, expCTHeaders, "", nil},
		//{"/t/myapp/myhotjason", respTypeJason, "GET", http.StatusOK, expCTHeaders, "", nil},
		//
		//{"/t/myapp/myroute", ok, "GET", http.StatusOK, expHeaders, "", nil},
		//{"/t/myapp/myerror", crasher, "GET", http.StatusBadGateway, expHeaders, "container exit code 2", nil},
		//{"/t/myapp/mydne", ``, "GET", http.StatusNotFound, nil, "pull access denied", nil},
		{"/t/myapp/mydnehot", ``, "GET", http.StatusNotFound, nil, "pull access denied", nil},
		//{"/t/myapp/mydneregistry", ``, "GET", http.StatusInternalServerError, nil, "connection refused", nil},
		//
		//{"/t/myapp/myoom", oomer, "GET", http.StatusBadGateway, nil, "container out of memory", nil},
		//{"/t/myapp/myhot", multiLog, "GET", http.StatusOK, nil, "", multiLogExpectHot},
		//{"/t/myapp/", multiLog, "GET", http.StatusOK, nil, "", multiLogExpectCold},
		//{"/t/myapp/mybigoutputjson", bigoutput, "GET", http.StatusBadGateway, nil, "function response too large", nil},
		//{"/t/myapp/mybigoutputjson", smalloutput, "GET", http.StatusOK, nil, "", nil},
		//{"/t/myapp/mybigoutputhttp", bigoutput, "GET", http.StatusBadGateway, nil, "", nil},
		//{"/t/myapp/mybigoutputhttp", smalloutput, "GET", http.StatusOK, nil, "", nil},
		//{"/t/myapp/mybigoutputcold", bigoutput, "GET", http.StatusBadGateway, nil, "", nil},
		//{"/t/myapp/mybigoutputcold", smalloutput, "GET", http.StatusOK, nil, "", nil},
	}

	callIds := make([]string, len(testCases))

	for i, test := range testCases {
		t.Run(fmt.Sprintf("Test_%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
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
		})

	}

	for i, test := range testCases {
		if test.expectedLogsSubStr != nil {
			if !checkLogs(t, i, ls, callIds[i], test.expectedLogsSubStr) {
				isFailure = true
			}
		}
	}
}

func TestTriggerRunnerTimeout(t *testing.T) {
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

	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
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
func TestTriggerRunnerMinimalConcurrentHotSync(t *testing.T) {
	buf := setLogBuffer()

	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
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
