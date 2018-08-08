package server

import (
	"testing"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
)

// TODO
//func TestRunnerInvokeEmptyBody(t *testing.T) {
//buf := setLogBuffer()
//isFailure := false

//defer func() {
//if isFailure {
//t.Log(buf.String())
//}
//}()

//rCfg := map[string]string{"ENABLE_HEADER": "yes", "ENABLE_FOOTER": "yes"} // enable container start/end header/footer
//rImg := "fnproject/fn-test-utils"

//app := &models.App{ID: "app_id", Name: "soup"}

//f1 := &models.Fn{ID: "1", Name: "guy", AppID: app.ID, Image: rImg, Format: "cloudevent", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 10, IdleTimeout: 20}, Config: rCfg}
//ds := datastore.NewMockInit(
//[]*models.App{app},
//[]*models.Fn{f1},
//)
//ls := logs.NewMock()

//rnr, cancelrnr := testRunner(t, ds, ls)
//defer cancelrnr()

//srv := testServer(ds, &mqs.Mock{}, ls, rnr, ServerTypeFull)

//emptyBody := `{"echoContent": "_TRX_ID_", "isDebug": true, "isEmptyBody": true}`

//// Test hot cases twice to rule out hot-containers corrupting next request.
//testCases := []struct {
//path string
//}{
//{"/invoke/1"},
//}

//for i, test := range testCases {
//t.Run(fmt.Sprintf("%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
//// TODO this should fail because the user must provide a cloud event
//})
//}
//}

func TestRunnerInvoke(t *testing.T) {
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

	// TODO test that other formats don't work (for now, at least)

	httpFn := &models.Fn{ID: "http_fn_id", Name: "http_fn", AppID: app.ID, Image: rImg, Format: "cloudevent", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	httpDneFn := &models.Fn{ID: "http_dne_fn_id", Name: "http_dne_fn", AppID: app.ID, Image: rImgBs1, Format: "cloudevent", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	httpDneRegistryFn := &models.Fn{ID: "http_dnereg_fn_id", Name: "http_dnereg_fn", AppID: app.ID, Image: rImgBs2, Format: "cloudevent", ResourceConfig: models.ResourceConfig{Memory: 64, Timeout: 30, IdleTimeout: 30}, Config: rCfg}
	oomFn := &models.Fn{ID: "http_fn_id", Name: "http_fn", AppID: app.ID, Image: rImg, Format: "cloudevent", ResourceConfig: models.ResourceConfig{Memory: 8, Timeout: 30, IdleTimeout: 30}, Config: rCfg}

	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Fn{httpDneRegistryFn, oomFn, httpFn, httpDneFn},
	)
	ls := logs.NewMock()

	rnr, cancelrnr := testRunner(t, ds, ls)
	defer cancelrnr()

	srv := testServer(ds, &mqs.Mock{}, ls, rnr, ServerTypeFull)

	expHeaders := map[string][]string{"Content-Type": {"application/cloudevents+json; charset=utf-8"}}

	_ = srv
	_ = expHeaders

	// Checking for EndOfLogs currently depends on scheduling of go-routines (in docker/containerd) that process stderr & stdout.
	// Therefore, not testing for EndOfLogs for hot containers (which has complex I/O processing) anymore.
	// multiLogExpectHot := []string{"BeginOfLogs" /*, "EndOfLogs" */}

	// TODO test cloud event version we don't support
	// TODO eventually validate required fields and test they are set
	// TODO test the actual output here not just the headers

	//crasher := `{"echoContent": "_TRX_ID_", "isDebug": true, "isCrash": true}`                      // crash container
	//oomer := `{"echoContent": "_TRX_ID_", "isDebug": true, "allocateMemory": 12000000}`             // ask for 12MB
	//badHot := `{"echoContent": "_TRX_ID_", "invalidResponse": true, "isDebug": true}`               // write a not json/http as output
	ok := ` {"cloudEventsVersion":"0.1","eventID":"1234","source":"","eventType":"","eventTypeVersion":"","eventTime":"2018-08-08T13:16:11.990897875Z","schemaURL":"","contentType":"application/json","data":{"echoContent": "_TRX_ID_", "isDebug": true}}` // good response / ok
	_ = ok
	//respTypeLie := `{"echoContent": "_TRX_ID_", "responseContentType": "foo/bar", "isDebug": true}` // Content-Type: foo/bar
	//respTypeJason := `{"echoContent": "_TRX_ID_", "jasonContentType": "foo/bar", "isDebug": true}`  // Content-Type: foo/bar

	// TODO finish this
	return

	// sleep between logs and with debug enabled, fn-test-utils will log header/footer below:
	//multiLog := `{"echoContent": "_TRX_ID_", "sleepTime": 1000, "isDebug": true}`
	//bigoutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1000}` // 1000 trailers to exceed 2K
	//smalloutput := `{"echoContent": "_TRX_ID_", "isDebug": true, "trailerRepeat": 1}`  // 1 trailer < 2K

	//testCases := []struct {
	//path               string
	//body               string
	//method             string
	//expectedCode       int
	//expectedHeaders    map[string][]string
	//expectedErrSubStr  string
	//expectedLogsSubStr []string
	//}{
	//{"/t/myapp/", ok, "GET", http.StatusOK, expHeaders, "", nil},

	//{"/t/myapp/myhot", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid http response", nil},
	//// hot container now back to normal:
	//{"/t/myapp/myhot", ok, "GET", http.StatusOK, expHeaders, "", nil},

	//{"/t/myapp/myhotjason", badHot, "GET", http.StatusBadGateway, expHeaders, "invalid json response", nil},
	//// hot container now back to normal:
	//{"/t/myapp/myhotjason", ok, "GET", http.StatusOK, expHeaders, "", nil},

	//{"/t/myapp/myhot", respTypeLie, "GET", http.StatusOK, expCTHeaders, "", nil},
	//{"/t/myapp/myhotjason", respTypeLie, "GET", http.StatusOK, expCTHeaders, "", nil},
	//{"/t/myapp/myhotjason", respTypeJason, "GET", http.StatusOK, expCTHeaders, "", nil},

	//{"/t/myapp/myroute", ok, "GET", http.StatusOK, expHeaders, "", nil},
	//{"/t/myapp/myerror", crasher, "GET", http.StatusBadGateway, expHeaders, "container exit code 2", nil},
	//{"/t/myapp/mydne", ``, "GET", http.StatusNotFound, nil, "pull access denied", nil},
	//{"/t/myapp/mydnehot", ``, "GET", http.StatusNotFound, nil, "pull access denied", nil},
	//{"/t/myapp/mydneregistry", ``, "GET", http.StatusInternalServerError, nil, "connection refused", nil},
	//{"/t/myapp/myoom", oomer, "GET", http.StatusBadGateway, nil, "container out of memory", nil},
	//{"/t/myapp/myhot", multiLog, "GET", http.StatusOK, nil, "", multiLogExpectHot},
	//{"/t/myapp/", multiLog, "GET", http.StatusOK, nil, "", multiLogExpectCold},
	//{"/t/myapp/mybigoutputjson", bigoutput, "GET", http.StatusBadGateway, nil, "function response too large", nil},
	//{"/t/myapp/mybigoutputjson", smalloutput, "GET", http.StatusOK, nil, "", nil},
	//{"/t/myapp/mybigoutputhttp", bigoutput, "GET", http.StatusBadGateway, nil, "", nil},
	//{"/t/myapp/mybigoutputhttp", smalloutput, "GET", http.StatusOK, nil, "", nil},
	//{"/t/myapp/mybigoutputcold", bigoutput, "GET", http.StatusBadGateway, nil, "", nil},
	//{"/t/myapp/mybigoutputcold", smalloutput, "GET", http.StatusOK, nil, "", nil},
	//}

	//callIds := make([]string, len(testCases))

	//for i, test := range testCases {
	//t.Run(fmt.Sprintf("Test_%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
	//trx := fmt.Sprintf("_trx_%d_", i)
	//body := strings.NewReader(strings.Replace(test.body, "_TRX_ID_", trx, 1))
	//_, rec := routerRequest(t, srv.Router, test.method, test.path, body)
	//respBytes, _ := ioutil.ReadAll(rec.Body)
	//respBody := string(respBytes)
	//maxBody := len(respBody)
	//if maxBody > 1024 {
	//maxBody = 1024
	//}

	//callIds[i] = rec.Header().Get("Fn_call_id")

	//if rec.Code != test.expectedCode {
	//isFailure = true
	//t.Errorf("Test %d: Expected status code to be %d but was %d. body: %s",
	//i, test.expectedCode, rec.Code, respBody[:maxBody])
	//}

	//if rec.Code == http.StatusOK && !strings.Contains(respBody, trx) {
	//isFailure = true
	//t.Errorf("Test %d: Expected response to include %s but got body: %s",
	//i, trx, respBody[:maxBody])

	//}

	//if test.expectedErrSubStr != "" && !strings.Contains(respBody, test.expectedErrSubStr) {
	//isFailure = true
	//t.Errorf("Test %d: Expected response to include %s but got body: %s",
	//i, test.expectedErrSubStr, respBody[:maxBody])

	//}

	//if test.expectedHeaders != nil {
	//for name, header := range test.expectedHeaders {
	//if header[0] != rec.Header().Get(name) {
	//isFailure = true
	//t.Errorf("Test %d: Expected header `%s` to be %s but was %s. body: %s",
	//i, name, header[0], rec.Header().Get(name), respBody)
	//}
	//}
	//}
	//})

	//}

	//for i, test := range testCases {
	//if test.expectedLogsSubStr != nil {
	//if !checkLogs(t, i, ls, callIds[i], test.expectedLogsSubStr) {
	//isFailure = true
	//}
	//}
	//}
}

//func TestTriggerRunnerTimeout(t *testing.T) {
//buf := setLogBuffer()
//isFailure := false

//// Log once after we are done, flow of events are important (hot/cold containers, idle timeout, etc.)
//// for figuring out why things failed.
//defer func() {
//if isFailure {
//t.Log(buf.String())
//}
//}()

//models.RouteMaxMemory = uint64(1024 * 1024 * 1024) // 1024 TB
//hugeMem := uint64(models.RouteMaxMemory - 1)

//app := &models.App{ID: "app_id", Name: "myapp", Config: models.Config{}}
//coldFn := &models.Fn{ID: "cold", Name: "cold", AppID: app.ID, Format: "", Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: 128, Timeout: 4, IdleTimeout: 30}}
//httpFn := &models.Fn{ID: "cold", Name: "http", AppID: app.ID, Format: "http", Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: 128, Timeout: 4, IdleTimeout: 30}}
//jsonFn := &models.Fn{ID: "json", Name: "json", AppID: app.ID, Format: "json", Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: 128, Timeout: 4, IdleTimeout: 30}}
//bigMemColdFn := &models.Fn{ID: "bigmemcold", Name: "bigmemcold", AppID: app.ID, Format: "", Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: hugeMem, Timeout: 4, IdleTimeout: 30}}
//bigMemHotFn := &models.Fn{ID: "bigmemhot", Name: "bigmemhot", AppID: app.ID, Format: "http", Image: "fnproject/fn-test-utils", ResourceConfig: models.ResourceConfig{Memory: hugeMem, Timeout: 4, IdleTimeout: 30}}

//ds := datastore.NewMockInit(
//[]*models.App{app},
//[]*models.Fn{coldFn, httpFn, jsonFn, bigMemColdFn, bigMemHotFn},
//[]*models.Trigger{
//{ID: "1", Name: "1", Source: "/cold", Type: "http", AppID: app.ID, FnID: coldFn.ID},
//{ID: "2", Name: "2", Source: "/hot", Type: "http", AppID: app.ID, FnID: httpFn.ID},
//{ID: "3", Name: "3", Source: "/hot-json", Type: "http", AppID: app.ID, FnID: jsonFn.ID},
//{ID: "4", Name: "4", Source: "/bigmem-cold", Type: "http", AppID: app.ID, FnID: bigMemColdFn.ID},
//{ID: "5", Name: "5", Source: "/bigmem-hot", Type: "http", AppID: app.ID, FnID: bigMemHotFn.ID},
//},
//)

//fnl := logs.NewMock()
//rnr, cancelrnr := testRunner(t, ds, fnl)
//defer cancelrnr()

//srv := testServer(ds, &mqs.Mock{}, fnl, rnr, ServerTypeFull)

//for i, test := range []struct {
//path            string
//body            string
//method          string
//expectedCode    int
//expectedHeaders map[string][]string
//}{
//{"/t/myapp/cold", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
//{"/t/myapp/cold", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
//{"/t/myapp/hot", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
//{"/t/myapp/hot", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
//{"/t/myapp/hot-json", `{"echoContent": "_TRX_ID_", "sleepTime": 5000, "isDebug": true}`, "POST", http.StatusGatewayTimeout, nil},
//{"/t/myapp/hot-json", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusOK, nil},
//{"/t/myapp/bigmem-cold", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusServiceUnavailable, map[string][]string{"Retry-After": {"15"}}},
//{"/t/myapp/bigmem-hot", `{"echoContent": "_TRX_ID_", "sleepTime": 0, "isDebug": true}`, "POST", http.StatusServiceUnavailable, map[string][]string{"Retry-After": {"15"}}},
//} {
//t.Run(fmt.Sprintf("%d_%s", i, strings.Replace(test.path, "/", "_", -1)), func(t *testing.T) {
//trx := fmt.Sprintf("_trx_%d_", i)
//body := strings.NewReader(strings.Replace(test.body, "_TRX_ID_", trx, 1))
//_, rec := routerRequest(t, srv.Router, test.method, test.path, body)
//respBytes, _ := ioutil.ReadAll(rec.Body)
//respBody := string(respBytes)
//maxBody := len(respBody)
//if maxBody > 1024 {
//maxBody = 1024
//}

//if rec.Code != test.expectedCode {
//isFailure = true
//t.Errorf("Test %d: Expected status code to be %d but was %d body: %#v",
//i, test.expectedCode, rec.Code, respBody[:maxBody])
//}

//if rec.Code == http.StatusOK && !strings.Contains(respBody, trx) {
//isFailure = true
//t.Errorf("Test %d: Expected response to include %s but got body: %s",
//i, trx, respBody[:maxBody])

//}

//if test.expectedHeaders != nil {
//for name, header := range test.expectedHeaders {
//if header[0] != rec.Header().Get(name) {
//isFailure = true
//t.Errorf("Test %d: Expected header `%s` to be %s but was %s body: %#v",
//i, name, header[0], rec.Header().Get(name), respBody[:maxBody])
//}
//}
//}
//})

//}
//}
