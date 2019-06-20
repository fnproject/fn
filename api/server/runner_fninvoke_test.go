package server

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/models"
)

func TestBadRequests(t *testing.T) {
	buf := setLogBuffer()
	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
	fn := &models.Fn{ID: "fn_id", AppID: "app_id"}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{fn},
	)
	rnr, cancel := testRunner(t, ds)
	defer cancel()
	srv := testServer(ds, rnr, ServerTypeFull)

	for i, test := range []struct {
		path          string
		contentType   string
		body          string
		expectedCode  int
		expectedError error
	}{
		{"/invoke/notfn", "", "", http.StatusNotFound, models.ErrFnsNotFound},
	} {
		request := createRequest(t, http.MethodPost, test.path, strings.NewReader(test.body))
		request.Header = map[string][]string{"Content-Type": []string{test.contentType}}
		_, rec := routerRequest2(t, srv.Router, request)

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

func TestFnInvokeRunnerExecEmptyBody(t *testing.T) {
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

	f1 := &models.Fn{ID: "hothttpstream", Name: "hothttpstream", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 10, IdleTimeout: 20}, Config: rCfg}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{f1},
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	srv := testServer(ds, rnr, ServerTypeFull)

	emptyBody := `{"echoContent": "_TRX_ID_", "isDebug": true, "isEmptyBody": true}`

	// Test hot cases twice to rule out hot-containers corrupting next request.
	testCases := []struct {
		path string
	}{
		{"/invoke/hothttpstream"},
		{"/invoke/hothttpstream"},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprintf("%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
			trx := fmt.Sprintf("_trx_%d_", i)
			body := strings.NewReader(strings.Replace(emptyBody, "_TRX_ID_", trx, 1))
			_, rec := routerRequest(t, srv.Router, http.MethodPost, test.path, body)
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

func TestFnInvokeRunnerExecution(t *testing.T) {
	buf := setLogBuffer()
	isFailure := false
	tweaker1 := envTweaker("FN_MAX_RESPONSE_SIZE", "2048")
	tweaker2 := envTweaker("FN_MAX_HDR_RESPONSE_SIZE", "1024")
	defer tweaker1()
	defer tweaker2()

	// Log once after we are done, flow of events are important (hot containers, idle timeout, etc.)
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

	models.MaxMemory = uint64(1024 * 1024 * 1024) // 1024 TB
	hugeMem := uint64(models.MaxMemory - 1)

	// quickly exit with exit code 0 without serving http/uds, or sleep 20 secs, then exit.. Two failure scenarios.
	failQuickCfg := map[string]string{"ENABLE_INIT_EXIT": "0"}
	failTimeoutCfg := map[string]string{"ENABLE_INIT_EXIT": "0", "ENABLE_INIT_DELAY_MSEC": "20000"}

	dneFn := &models.Fn{ID: "dne_fn_id", Name: "dne_fn", AppID: app.ID, Image: rImgBs1, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	dneRegistryFn := &models.Fn{ID: "dnereg_fn_id", Name: "dnereg_fn", AppID: app.ID, Image: rImgBs2, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	httpStreamFn := &models.Fn{ID: "http_stream_fn_id", Name: "http_stream_fn", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	bigMemHotFn := &models.Fn{ID: "bigmem", Name: "bigmemhot", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: hugeMem, Timeout: 4, IdleTimeout: 30}}
	failQuickFn := &models.Fn{ID: "fail_fn_quick", Name: "fail_fn_quick", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: failQuickCfg}
	failTimeoutFn := &models.Fn{ID: "fail_fn_timeout", Name: "fail_fn_timeout", AppID: app.ID, Image: rImg, ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: failTimeoutCfg}

	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{dneFn, dneRegistryFn, httpStreamFn, failQuickFn, failTimeoutFn, bigMemHotFn},
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	srv := testServer(ds, rnr, ServerTypeFull, LimitRequestBody(32256))

	inStripHeaders := map[string][]string{"Keep-Alive": {"true"}}

	expHeaders := map[string][]string{"Content-Type": {"application/json; charset=utf-8"}}
	expCTHeaders := map[string][]string{"Content-Type": {"foo/bar"}}
	expStripHeaders := map[string][]string{"Keep-Alive": {""}}

	// Checking for EndOfLogs currently depends on scheduling of go-routines (in docker/containerd) that process stderr & stdout.
	// Therefore, not testing for EndOfLogs for hot containers (which has complex I/O processing) anymore.
	multiLogExpectHot := []string{"BeginOfLogs" /*, "EndOfLogs" */}

	crasher := `{"echoContent": "_TRX_ID_", "isDebug": true, "isCrash": true}`           // crash container
	oomer := `{"echoContent": "_TRX_ID_", "isDebug": true, "allocateMemory": 120000000}` // ask for 120MB
	// XXX(reed): do we have an invalid http response? no right?
	ok := `{"echoContent": "_TRX_ID_", "responseContentType": "application/json; charset=utf-8", "isDebug": true}` // good response / ok
	respTypeLie := `{"echoContent": "_TRX_ID_", "responseContentType": "foo/bar", "isDebug": true}`                // Content-Type: foo/bar

	// sleep between logs and with debug enabled, fn-test-utils will log header/footer below:
	multiLog := `{"echoContent": "_TRX_ID_", "sleepTime": 1000, "isDebug": true}`
	//over sized request
	var bigbufa [32257]byte
	rand.Read(bigbufa[:])
	bigbuf := base64.StdEncoding.EncodeToString(bigbufa[:])                            // this will be > bigbufa, but json compatible
	bigoutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1000}` // 1000 trailers to exceed 2K

	bighdroutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "returnHeaders": {"zoo": ["` + strings.Repeat("a", 1024) + `"]}}` // big header to exceed
	striphdr := `{"echoContent": "_TRX_ID_", "isDebug": true, "returnHeaders": {"Keep-Alive": ["true"]}}`                           // this should get stripped
	striphdrin := `{"echoContent": "_TRX_ID_", "isDebug": true, "expectHeaders": {"Keep-Alive": [""]}}`                             // this should get stripped

	smalloutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "responseContentType":"application/json; charset=utf-8", "trailerRepeat": 1}` // 1 trailer < 2K

	testCases := []struct {
		path               string
		body               string
		headers            map[string][]string
		method             string
		expectedCode       int
		expectedHeaders    map[string][]string
		expectedErrSubStr  string
		expectedLogsSubStr []string
	}{
		{"/invoke/http_stream_fn_id", ok, nil, http.MethodPost, http.StatusOK, expHeaders, "", nil},
		// NOTE: nil, we can't test bad response framing anymore easily (eg invalid http response), should we even worry about it?
		{"/invoke/http_stream_fn_id", respTypeLie, nil, http.MethodPost, http.StatusOK, expCTHeaders, "", nil},
		{"/invoke/http_stream_fn_id", crasher, nil, http.MethodPost, http.StatusBadGateway, expHeaders, "error receiving function response", nil},
		// XXX(reed): nil, we could stop buffering function responses so that we can stream things?
		{"/invoke/http_stream_fn_id", bighdroutput, nil, http.MethodPost, http.StatusBadGateway, nil, "function response header too large", nil},
		{"/invoke/http_stream_fn_id", striphdr, nil, http.MethodPost, http.StatusOK, expStripHeaders, "", nil},
		{"/invoke/http_stream_fn_id", striphdrin, inStripHeaders, http.MethodPost, http.StatusOK, nil, "", nil},
		{"/invoke/http_stream_fn_id", bigoutput, nil, http.MethodPost, http.StatusBadGateway, nil, "function response too large", nil},
		{"/invoke/http_stream_fn_id", smalloutput, nil, http.MethodPost, http.StatusOK, expHeaders, "", nil},
		// XXX(reed): nil, meh we really should try to get oom out, but maybe it's better left to the logs?
		{"/invoke/http_stream_fn_id", oomer, nil, http.MethodPost, http.StatusBadGateway, nil, "error receiving function response", nil},
		{"/invoke/http_stream_fn_id", bigbuf, nil, http.MethodPost, http.StatusRequestEntityTooLarge, nil, "", nil},

		{"/invoke/dne_fn_id", ``, nil, http.MethodPost, http.StatusNotFound, nil, "Failed to pull image", nil},
		{"/invoke/dnereg_fn_id", ``, nil, http.MethodPost, http.StatusBadGateway, nil, "connection refused", nil},

		// XXX(reed): nil, nil, what are these?
		{"/invoke/http_stream_fn_id", multiLog, nil, http.MethodPost, http.StatusOK, nil, "", multiLogExpectHot},

		{"/invoke/fail_fn_quick", ok, nil, http.MethodPost, http.StatusBadGateway, nil, "container failed to initialize", nil},
		{"/invoke/fail_fn_timeout", ok, nil, http.MethodPost, http.StatusGatewayTimeout, nil, "Container initialization timed out", nil},
		{"/invoke/fn_id", ok, nil, http.MethodPut, http.StatusMethodNotAllowed, nil, "Method not allowed", nil},

		{"/invoke/bigmem", ok, nil, http.MethodPost, http.StatusBadRequest, nil, "cannot be allocated", nil},
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
			cid := callIds[i]

			if rec.Code == 200 && cid == "" {
				isFailure = true
				t.Errorf("Test %d call_id %s: Expected successful call id to be non-empty but was %s. body: %s",
					i, cid, cid, respBody[:maxBody])
			}

			if rec.Code != test.expectedCode {
				isFailure = true
				t.Errorf("Test %d call_id %s: Expected status code to be %d but was %d. body: %s",
					i, cid, test.expectedCode, rec.Code, respBody[:maxBody])
			}

			if rec.Code == http.StatusOK && !strings.Contains(respBody, trx) {
				isFailure = true
				t.Errorf("Test %d call_id %s: Expected response to include %s but got body: %s",
					i, cid, trx, respBody[:maxBody])

			}

			if test.expectedErrSubStr != "" && !strings.Contains(respBody, test.expectedErrSubStr) {
				isFailure = true
				t.Errorf("Test %d call_id %s: Expected response to include %s but got body: %s",
					i, cid, test.expectedErrSubStr, respBody[:maxBody])

			}

			if test.expectedHeaders != nil {
				for name, header := range test.expectedHeaders {
					if header[0] != rec.Header().Get(name) {
						isFailure = true
						t.Errorf("Test %d call_id %s: Expected header `%s` to be %s but was %s. body: %s",
							i, cid, name, header[0], rec.Header().Get(name), respBody)
					}
				}
			}
		})

	}

	for i, test := range testCases {
		if test.expectedLogsSubStr != nil {
			if !checkLogs(t, i, callIds[i], test.expectedLogsSubStr) {
				isFailure = true
			}
		}
	}
}

func TestInvokeRunnerTimeout(t *testing.T) {
	buf := setLogBuffer()
	isFailure := false

	// Log once after we are done, flow of events are important (hot containers, idle timeout, etc.)
	// for figuring out why things failed.
	defer func() {
		if isFailure {
			t.Log(buf.String())
		}
	}()

	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
	httpStreamFn := &models.Fn{ID: "http-stream", Name: "http-stream", AppID: app.ID, Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: 128, Timeout: 4, IdleTimeout: 30}}

	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{httpStreamFn},
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	srv := testServer(ds, rnr, ServerTypeFull)

	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/invoke/http-stream", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, http.MethodPost, http.StatusGatewayTimeout, nil},
		{"/invoke/http-stream", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, http.MethodPost, http.StatusOK, nil},
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

			cid := rec.Header().Get("Fn-Call-Id")
			if cid == "" {
				isFailure = true
				t.Errorf("Test %d call_id %s: Expected call id to be non-empty but was %s. body: %s",
					i, cid, cid, respBody[:maxBody])
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
func TestInvokeRunnerMinimalConcurrentHotSync(t *testing.T) {
	buf := setLogBuffer()

	app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
	fn := &models.Fn{ID: "fn_id", AppID: app.ID, Name: "myfn", Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: 128, Timeout: 30, IdleTimeout: 5}}
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{fn},
	)

	rnr, cancelrnr := testRunner(t, ds)
	defer cancelrnr()

	srv := testServer(ds, rnr, ServerTypeFull)

	for i, test := range []struct {
		path            string
		body            string
		method          string
		expectedCode    int
		expectedHeaders map[string][]string
	}{
		{"/invoke/fn_id", `{"sleepTime": 100, "isDebug": true}`, http.MethodPost, http.StatusOK, nil},
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
