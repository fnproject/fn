package server

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"context"
	"os"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"reflect"
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
	ls := logs.NewMock()
	var mq models.MessageQueue = &mqs.Mock{}
	for _, a := range args {
		switch arg := a.(type) {
		case models.MessageQueue:
			mq = arg
		case models.LogStore:
			ls = arg
		}
	}
	r := agent.New(agent.NewDirectCallDataAccess(ls, mq))
	return r, func() { r.Close() }
}

func checkLogs(t *testing.T, tnum int, ds models.LogStore, callID string, expected []string) bool {

	logReader, err := ds.GetLog(context.Background(), "fnid_not_needed_by_mock", callID)
	if err != nil {
		t.Errorf("Test %d: GetLog for call_id:'%s' returned err %s",
			tnum, callID, err.Error())
		return false
	}

	logBytes, err := ioutil.ReadAll(logReader)
	if err != nil {
		t.Errorf("Test %d: GetLog read IO call_id:'%s' returned err %s",
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

	f1 := &models.Fn{ID: "hot", Name: "hot", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 10, IdleTimeout: 20}, Config: rCfg}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{f1},
		[]*models.Trigger{
			{ID: "t2", Name: "t2", AppID: app.ID, FnID: f1.ID, Type: "http", Source: "/hot"},
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
		{"/t/soup/hot"},
		{"/t/soup/hot"},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprintf("%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
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
		})
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

	dneFn := &models.Fn{ID: "default_dne_fn_id", Name: "default_dne_fn", AppID: app.ID, Image: rImgBs1, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	dneRegistryFn := &models.Fn{ID: "http_dnereg_fn_id", Name: "http_dnereg_fn", AppID: app.ID, Image: rImgBs2, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	httpStreamFn := &models.Fn{ID: "http_stream_fn_id", Name: "http_stream_fn", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}

	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{dneFn, dneRegistryFn, httpStreamFn},
		[]*models.Trigger{
			{ID: "6", Name: "6", Source: "/mydne", Type: "http", AppID: app.ID, FnID: dneFn.ID},
			{ID: "8", Name: "8", Source: "/mydneregistry", Type: "http", AppID: app.ID, FnID: dneRegistryFn.ID},
			{ID: "13", Name: "13", Source: "/httpstream", Type: "http", AppID: app.ID, FnID: httpStreamFn.ID},
		},
	)
	ls := logs.NewMock()

	rnr, cancelrnr := testRunner(t, ds, ls)
	defer cancelrnr()

	srv := testServer(ds, &mqs.Mock{}, ls, rnr, ServerTypeFull)

	expHeaders := map[string][]string{"Content-Type": {"application/json; charset=utf-8"}}
	expCTHeaders := map[string][]string{"Content-Type": {"foo/bar"}}

	// Checking for EndOfLogs currently depends on scheduling of go-routines (in docker/containerd) that process stderr & stdout.
	// Therefore, not testing for EndOfLogs for hot containers (which has complex I/O processing) anymore.
	multiLogExpectHot := []string{"BeginOfLogs" /*, "EndOfLogs" */}

	crasher := `{"echoContent": "_TRX_ID_", "isDebug": true, "isCrash": true}`                                     // crash container
	oomer := `{"echoContent": "_TRX_ID_", "isDebug": true, "allocateMemory": 120000000}`                           // ask for 120MB
	ok := `{"echoContent": "_TRX_ID_", "responseContentType": "application/json; charset=utf-8", "isDebug": true}` // good response / ok
	respTypeLie := `{"echoContent": "_TRX_ID_", "responseContentType": "foo/bar", "isDebug": true}`                // Content-Type: foo/bar

	// sleep between logs and with debug enabled, fn-test-utils will log header/footer below:
	multiLog := `{"echoContent": "_TRX_ID_", "sleepTime": 1000, "isDebug": true}`
	bigoutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1000}`                                                          // 1000 trailers to exceed 2K
	smalloutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1, "responseContentType": "application/json; charset=utf-8"}` // 1 trailer < 2K

	statusChecker := `{"echoContent": "_TRX_ID_", "isDebug": true, "responseCode":202, "responseContentType": "application/json; charset=utf-8"}`

	// these tests are such a pita it's easier to comment most of them out. instead of fixing it i'm doing this fuck me yea
	_, _, _, _, _, _, _, _, _, _, _ = expHeaders, expCTHeaders, multiLogExpectHot, crasher, oomer, ok, respTypeLie, multiLog, bigoutput, smalloutput, statusChecker

	fooHeader := map[string][]string{"Content-Type": {"application/hateson"}, "Test-Header": {"foo"}}
	expFooHeaders := map[string][]string{"Content-Type": {"application/hateson"}, "Return-Header": {"foo", "bar"}}
	expFooHeadersBody := `{"echoContent": "_TRX_ID_",
		"expectHeaders": {
			"Content-Type":["application/hateson"],
			"Test-Header":["foo"]
		},
		"returnHeaders": {
			"Return-Header":["foo","bar"]
		},
		"responseContentType":"application/hateson",
		"isDebug": true}`

	testCases := []struct {
		path               string
		headers            map[string][]string
		body               string
		method             string
		expectedCode       int
		expectedHeaders    map[string][]string
		expectedErrSubStr  string
		expectedLogsSubStr []string
	}{
		// XXX(reed): we test a lot of stuff in invoke, we really only need to test headers / status code here dude...
		{"/t/myapp/httpstream", nil, ok, "POST", http.StatusOK, expHeaders, "", nil},
		{"/t/myapp/httpstream", nil, statusChecker, "POST", 202, expHeaders, "", nil},
		{"/t/myapp/httpstream", fooHeader, expFooHeadersBody, "POST", http.StatusOK, expFooHeaders, "", nil},
		// NOTE: we can't test bad response framing anymore easily (eg invalid http response), should we even worry about it?
		{"/t/myapp/httpstream", nil, respTypeLie, "POST", http.StatusOK, expCTHeaders, "", nil},
		{"/t/myapp/httpstream", nil, crasher, "POST", http.StatusBadGateway, expHeaders, "error receiving function response", nil},
		// XXX(reed): we could stop buffering function responses so that we can stream things?
		{"/t/myapp/httpstream", nil, bigoutput, "POST", http.StatusBadGateway, nil, "function response too large", nil},
		{"/t/myapp/httpstream", nil, smalloutput, "POST", http.StatusOK, expHeaders, "", nil},
		// XXX(reed): meh we really should try to get oom out, but maybe it's better left to the logs?
		{"/t/myapp/httpstream", nil, oomer, "POST", http.StatusBadGateway, nil, "error receiving function response", nil},

		{"/t/myapp/mydne", nil, ``, "GET", http.StatusNotFound, nil, "pull access denied", nil},
		{"/t/myapp/mydneregistry", nil, ``, "GET", http.StatusInternalServerError, nil, "connection refused", nil},

		// XXX(reed): what are these?
		{"/t/myapp/httpstream", nil, multiLog, "GET", http.StatusOK, nil, "", multiLogExpectHot},
	}

	callIds := make([]string, len(testCases))

	for i, test := range testCases {
		t.Run(fmt.Sprintf("Test_%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
			trx := fmt.Sprintf("_trx_%d_", i)
			body := strings.NewReader(strings.Replace(test.body, "_TRX_ID_", trx, 1))
			req := createRequest(t, test.method, test.path, body)
			if test.headers != nil {
				req.Header = test.headers
			}
			_, rec := routerRequest2(t, srv.Router, req)
			respBytes, _ := ioutil.ReadAll(rec.Body)
			respBody := string(respBytes)
			maxBody := len(respBody)
			if maxBody > 1024 {
				maxBody = 1024
			}

			callIds[i] = rec.Header().Get("Fn-Call-Id")

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
					if !reflect.DeepEqual(header, rec.Header()[name]) {
						isFailure = true
						t.Errorf("Test %d: Expected header `%s` to be `%v` but was `%v`. body: `%s`",
							i, name, header, rec.Header()[name], respBody)
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

	models.MaxMemory = uint64(1024 * 1024 * 1024) // 1024 TB
	hugeMem := uint64(models.MaxMemory - 1)

	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
	fn := &models.Fn{ID: "hot", Name: "hot", AppID: app.ID, Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: 128, Timeout: 4, IdleTimeout: 30}}
	bigMemHotFn := &models.Fn{ID: "bigmemhot", Name: "bigmemhot", AppID: app.ID, Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: hugeMem, Timeout: 4, IdleTimeout: 30}}

	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{fn, bigMemHotFn},
		[]*models.Trigger{
			{ID: "2", Name: "2", Source: "/hot", Type: "http", AppID: app.ID, FnID: fn.ID},
			{ID: "5", Name: "5", Source: "/bigmem-hot", Type: "http", AppID: app.ID, FnID: bigMemHotFn.ID},
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
		{"/t/myapp/hot", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
		{"/t/myapp/hot", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
		{"/t/myapp/bigmem-hot", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusBadRequest, nil},
	} {
		t.Run(fmt.Sprintf("%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
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
		})
	}
}

// Minimal test that checks the possibility of invoking concurrent hot sync functions.
func TestTriggerRunnerMinimalConcurrentHotSync(t *testing.T) {
	buf := setLogBuffer()

	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
	fn := &models.Fn{ID: "fn_id", AppID: app.ID, Name: "myfn", Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: 128, Timeout: 30, IdleTimeout: 5}}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{fn},
		[]*models.Trigger{{Name: "1", Source: "/hot", AppID: app.ID, FnID: fn.ID, Type: "http"}},
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
		{"/t/myapp/hot", `{"sleepTime": 100, "isDebug": true}`, "POST", http.StatusOK, nil},
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
