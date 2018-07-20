package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/sirupsen/logrus"
)

func init() {
	// TODO figure out some sane place to stick this
	formatter := &logrus.TextFormatter{
		FullTimestamp: true,
	}
	logrus.SetFormatter(formatter)
	logrus.SetLevel(logrus.DebugLevel)
}

// TODO need to add at least one test for our cachy cache

func checkExpectedHeaders(t *testing.T, expectedHeaders http.Header, receivedHeaders http.Header) {

	checkMap := make([]string, 0, len(expectedHeaders))
	for k := range expectedHeaders {
		checkMap = append(checkMap, k)
	}

	for k, vs := range receivedHeaders {
		for i, v := range expectedHeaders[k] {
			if i >= len(vs) || vs[i] != v {
				t.Fatal("header mismatch", k, vs)
			}
		}

		for i := range checkMap {
			if checkMap[i] == k {
				checkMap = append(checkMap[:i], checkMap[i+1:]...)
				break
			}
		}
	}

	if len(checkMap) > 0 {
		t.Fatalf("expected headers not found=%v", checkMap)
	}
}

func checkClose(t *testing.T, a Agent) {
	if err := a.Close(); err != nil {
		t.Fatalf("Failed to close agent: %v", err)
	}
}

func TestCallConfigurationRequest(t *testing.T) {
	appName := "myapp"
	path := "/"
	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	typ := "sync"
	format := "default"

	cfg := models.Config{"APP_VAR": "FOO"}
	rCfg := models.Config{"ROUTE_VAR": "BAR"}

	app := &models.App{ID: "app_id", Name: appName, Config: cfg}
	route := &models.Route{
		AppID:       app.ID,
		Config:      rCfg,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
	}

	ls := logs.NewMock()

	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	w := httptest.NewRecorder()

	method := "GET"
	url := "http://127.0.0.1:8080/r/" + appName + path
	payload := "payload"
	contentLength := strconv.Itoa(len(payload))
	req, err := http.NewRequest(method, url, strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("MYREALHEADER", "FOOLORD")
	req.Header.Add("MYREALHEADER", "FOOPEASANT")
	req.Header.Add("Content-Length", contentLength)
	req.Header.Add("FN_PATH", "thewrongroute") // ensures that this doesn't leak out, should be overwritten

	call, err := a.GetCall(req.Context(),
		WithWriter(w), // XXX (reed): order matters [for now]
		FromRouteEvent(app, route, req),
	)
	if err != nil {
		t.Fatal(err)
	}

	model := call.Model()

	// make sure the values are all set correctly
	if model.ID == "" {
		t.Fatal("model does not have id, GetCall should assign id")
	}
	if model.AppID != app.ID {
		t.Fatal("app ID mismatch", model.ID, app.ID)
	}
	if model.Path != path {
		t.Fatal("path mismatch", model.Path, path)
	}
	if model.Image != image {
		t.Fatal("image mismatch", model.Image, image)
	}
	if model.Type != "sync" {
		t.Fatal("route type mismatch", model.Type)
	}
	if model.Priority == nil {
		t.Fatal("GetCall should make priority non-nil so that async works because for whatever reason some clowns plumbed it all over the mqs even though the user can't specify it gg")
	}
	if model.Timeout != timeout {
		t.Fatal("timeout mismatch", model.Timeout, timeout)
	}
	if model.IdleTimeout != idleTimeout {
		t.Fatal("idle timeout mismatch", model.IdleTimeout, idleTimeout)
	}
	if time.Time(model.CreatedAt).IsZero() {
		t.Fatal("GetCall should stamp CreatedAt, got nil timestamp")
	}
	if model.URL != url {
		t.Fatal("url mismatch", model.URL, url)
	}
	if model.Method != method {
		t.Fatal("method mismatch", model.Method, method)
	}

	expectedConfig := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": appName,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
	}

	for k, v := range expectedConfig {
		if v2 := model.Config[k]; v2 != v {
			t.Fatal("config mismatch", k, v, v2, model.Config)
		}
		delete(expectedConfig, k)
	}

	if len(expectedConfig) > 0 {
		t.Fatal("got extra vars in config set, add me to tests ;)", expectedConfig)
	}

	expectedHeaders := make(http.Header)

	expectedHeaders.Add("MYREALHEADER", "FOOLORD")
	expectedHeaders.Add("MYREALHEADER", "FOOPEASANT")
	expectedHeaders.Add("Content-Length", contentLength)

	checkExpectedHeaders(t, expectedHeaders, model.Headers)

	// TODO check response writer for route headers
}

func TestCallConfigurationModel(t *testing.T) {
	app := &models.App{Name: "myapp"}

	path := "/"
	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	CPUs := models.MilliCPUs(1000)
	method := "GET"
	url := "http://127.0.0.1:8080/r/" + app.Name + path
	payload := "payload"
	typ := "sync"
	format := "default"
	cfg := models.Config{
		"FN_FORMAT":   format,
		"FN_APP_NAME": app.Name,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_CPUS":     CPUs.String(),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
	}

	ctx := context.Background()

	cm := &models.Call{
		AppID:       app.ID,
		Config:      cfg,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		CPUs:        CPUs,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	// FromModel doesn't need a datastore, for now...
	ls := logs.NewMock()

	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	callI, err := a.GetCall(ctx, FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	if callI.Model().Payload != payload {
		t.Fatal("expected payload to match, but it was a lie")
	}
}

func TestAsyncCallHeaders(t *testing.T) {
	app := &models.App{ID: "app_id", Name: "myapp"}

	path := "/"
	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	CPUs := models.MilliCPUs(200)
	method := "GET"
	url := "http://127.0.0.1:8080/r/" + app.Name + path
	payload := "payload"
	typ := "async"
	format := "http"
	contentType := "suberb_type"
	contentLength := strconv.FormatInt(int64(len(payload)), 10)
	config := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": app.Name,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_CPUS":     CPUs.String(),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
		"DOUBLE_VAR":  "BIZ, BAZ",
	}
	headers := map[string][]string{
		// FromRouteEvent would insert these from original HTTP request
		"Content-Type":   {contentType},
		"Content-Length": {contentLength},
	}

	cm := &models.Call{
		AppID:       app.ID,
		Config:      config,
		Headers:     headers,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		CPUs:        CPUs,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	ctx := context.Background()

	// FromModel doesn't need a datastore, for now...
	ls := logs.NewMock()

	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	callI, err := a.GetCall(ctx, FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	// These should be here based on payload length and/or fn_header_* original headers
	expectedHeaders := make(http.Header)
	expectedHeaders.Set("Content-Type", contentType)
	expectedHeaders.Set("Content-Length", strconv.FormatInt(int64(len(payload)), 10))

	checkExpectedHeaders(t, expectedHeaders, http.Header(callI.Model().Headers))

	if callI.Model().Payload != payload {
		t.Fatal("expected payload to match, but it was a lie")
	}
}

func TestLoggerIsStringerAndWorks(t *testing.T) {
	// TODO test limit writer, logrus writer, etc etc

	var call models.Call
	logger := setupLogger(context.Background(), 1*1024*1024, &call)

	if _, ok := logger.(fmt.Stringer); !ok {
		// NOTE: if you are reading, maybe what you've done is ok, but be aware we were relying on this for optimization...
		t.Fatal("you turned the logger into something inefficient and possibly better all at the same time, how dare ye!")
	}

	str := "0 line\n1 line\n2 line\n\n4 line"
	logger.Write([]byte(str))

	strGot := logger.(fmt.Stringer).String()

	if strGot != str {
		t.Fatal("logs did not match expectations, like being an adult", strGot, str)
	}

	logger.Close() // idk maybe this would panic might as well ca this

	// TODO we could check for the toilet to flush here to logrus
}

func TestLoggerTooBig(t *testing.T) {

	var call models.Call
	logger := setupLogger(context.Background(), 10, &call)

	str := fmt.Sprintf("0 line\n1 l\n-----max log size 10 bytes exceeded, truncating log-----\n")

	n, err := logger.Write([]byte(str))
	if err != nil {
		t.Fatalf("err returned, but should not fail err=%v n=%d", err, n)
	}
	if n != len(str) {
		t.Fatalf("n should be %d, but got=%d", len(str), n)
	}

	// oneeeeee moreeee time... (cue in Daft Punk), the results appear as if we wrote
	// again... But only "limit" bytes should succeed, ignoring the subsequent writes...
	n, err = logger.Write([]byte(str))
	if err != nil {
		t.Fatalf("err returned, but should not fail err=%v n=%d", err, n)
	}
	if n != len(str) {
		t.Fatalf("n should be %d, but got=%d", len(str), n)
	}

	strGot := logger.(fmt.Stringer).String()

	if strGot != str {
		t.Fatalf("logs did not match expectations, like being an adult got=\n%v\nexpected=\n%v\n", strGot, str)
	}

	logger.Close()
}

type testListener struct {
	afterCall func(context.Context, *models.Call) error
}

func (l testListener) AfterCall(ctx context.Context, call *models.Call) error {
	return l.afterCall(ctx, call)
}
func (l testListener) BeforeCall(context.Context, *models.Call) error {
	return nil
}

func TestSubmitError(t *testing.T) {
	app := &models.App{Name: "myapp"}

	path := "/"
	image := "fnproject/fn-test-utils"
	const timeout = 10
	const idleTimeout = 20
	const memory = 256
	CPUs := models.MilliCPUs(200)
	method := "GET"
	url := "http://127.0.0.1:8080/r/" + app.Name + path
	payload := `{"sleepTime": 0, "isDebug": true, "isCrash": true}`
	typ := "sync"
	format := "default"
	config := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": app.Name,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_CPUS":     CPUs.String(),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
		"DOUBLE_VAR":  "BIZ, BAZ",
	}

	cm := &models.Call{
		AppID:       app.ID,
		Config:      config,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		CPUs:        CPUs,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	ctx := context.Background()

	// FromModel doesn't need a datastore, for now...
	ls := logs.NewMock()

	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	var wg sync.WaitGroup
	wg.Add(1)

	afterCall := func(ctx context.Context, call *models.Call) error {
		defer wg.Done()
		if cm.Status != "error" {
			t.Fatal("expected status to be set to 'error' but was", cm.Status)
		}

		if cm.Error == "" {
			t.Fatal("expected error string to be set on call")
		}
		return nil
	}

	a.AddCallListener(&testListener{afterCall: afterCall})

	callI, err := a.GetCall(ctx, FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(ctx, callI)
	if err == nil {
		t.Fatal("expected error but got none")
	}

	wg.Wait()
}

// this implements io.Reader, but importantly, is not a strings.Reader or
// a type of reader than NewRequest can identify to set the content length
// (important, for some tests)
type dummyReader struct {
	io.Reader
}

func TestHTTPWithoutContentLengthWorks(t *testing.T) {
	// TODO it may be a good idea to mock out the http server and use a real
	// response writer with sync, and also test that this works with async + log

	appName := "myapp"
	path := "/hello"
	url := "http://127.0.0.1:8080/r/" + appName + path

	app := &models.App{ID: "app_id", Name: appName}
	route := &models.Route{
		Path:        path,
		AppID:       app.ID,
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Format:      "http", // this _is_ the test
		Timeout:     5,
		IdleTimeout: 10,
		Memory:      128,
	}

	ls := logs.NewMock()
	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	bodOne := `{"echoContent":"yodawg"}`

	// get a req that uses the dummy reader, so that this can't determine
	// the size of the body and set content length (user requests may also
	// forget to do this, and we _should_ read it as chunked without issue).
	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(bodOne)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}
	ctx := req.Context()

	// grab a buffer so we can read what gets written to this guy
	var out bytes.Buffer
	callI, err := a.GetCall(ctx, FromRouteEvent(app, route, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(ctx, callI)
	if err != nil {
		t.Error("submit should not error:", err)
	}

	// we're using http format so this will have written a whole http request
	res, err := http.ReadResponse(bufio.NewReader(&out), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	// {"request":{"echoContent":"yodawg"}}
	var resp struct {
		R struct {
			Body string `json:"echoContent"`
		} `json:"request"`
	}

	json.NewDecoder(res.Body).Decode(&resp)

	if resp.R.Body != "yodawg" {
		t.Fatal(`didn't get a yodawg in the body, http protocol may be fudged up
			(to debug, recommend ensuring inside the function gets 'Transfer-Encoding: chunked' if
			no Content-Length is set. also make sure the body makes it (and the image hasn't changed)); GLHF, got:`, resp.R.Body)
	}
}

func TestGetCallReturnsResourceImpossibility(t *testing.T) {
	call := &models.Call{
		AppID:       id.New().String(),
		Path:        "/yoyo",
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Format:      "http",
		Timeout:     1,
		IdleTimeout: 2,
		Memory:      math.MaxUint64,
	}

	ls := logs.NewMock()
	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	ctx := context.Background()

	_, err := a.GetCall(ctx, FromModel(call))
	if err != models.ErrCallTimeoutServerBusy {
		t.Fatal("did not get expected err, got: ", err)
	}
}

//
// Tmp directory should be RW by default.
//
func TestTmpFsRW(t *testing.T) {
	appName := "myapp"
	path := "/hello"
	url := "http://127.0.0.1:8080/r/" + appName + path

	app := &models.App{ID: "app_id", Name: appName}

	route := &models.Route{
		Path:        path,
		AppID:       app.ID,
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Format:      "http", // this _is_ the test
		Timeout:     5,
		IdleTimeout: 10,
		Memory:      128,
	}

	ls := logs.NewMock()
	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	// Here we tell fn-test-utils to read file /proc/mounts and create a /tmp/salsa of 4MB
	bodOne := `{"readFile":"/proc/mounts", "createFile":"/tmp/salsa", "createFileSize": 4194304, "isDebug": true}`

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(bodOne)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	ctx := req.Context()

	var out bytes.Buffer
	callI, err := a.GetCall(ctx, FromRouteEvent(app, route, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(ctx, callI)
	if err != nil {
		t.Error("submit should not error:", err)
	}

	// we're using http format so this will have written a whole http request
	res, err := http.ReadResponse(bufio.NewReader(&out), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	// Let's fetch read output and write results. See fn-test-utils AppResponse struct (data field)
	var resp struct {
		R struct {
			MountsRead string `json:"/proc/mounts.read_output"`
			CreateFile string `json:"/tmp/salsa.create_error"`
		} `json:"data"`
	}

	json.NewDecoder(res.Body).Decode(&resp)

	// Let's check what mounts are on...
	mounts := strings.Split(resp.R.MountsRead, "\n")
	isFound := false
	isRootFound := false
	for _, mnt := range mounts {
		tokens := strings.Split(mnt, " ")
		if len(tokens) < 3 {
			continue
		}

		point := tokens[1]
		opts := tokens[3]

		// tmp dir with RW and no other options (size, inodes, etc.)
		if point == "/tmp" && opts == "rw,nosuid,nodev,noexec,relatime" {
			// good
			isFound = true
		} else if point == "/" && strings.HasPrefix(opts, "ro,") {
			// Read-only root, good...
			isRootFound = true
		}
	}

	if !isFound || !isRootFound {
		t.Fatal(`didn't get proper mounts for /tmp or /, got /proc/mounts content of:\n`, resp.R.MountsRead)
	}

	// write file should not have failed...
	if resp.R.CreateFile != "" {
		t.Fatal(`limited tmpfs should generate fs full error, but got output: `, resp.R.CreateFile)
	}
}

func TestTmpFsSize(t *testing.T) {
	appName := "myapp"
	path := "/hello"
	url := "http://127.0.0.1:8080/r/" + appName + path

	app := &models.App{ID: "app_id", Name: appName}

	route := &models.Route{
		Path:        path,
		AppID:       app.ID,
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Format:      "http", // this _is_ the test
		Timeout:     5,
		IdleTimeout: 10,
		Memory:      64,
		TmpFsSize:   1,
	}

	cfg, err := NewConfig()
	if err != nil {
		t.Fatal(err)
	}

	cfg.MaxTmpFsInodes = 1024

	ls := logs.NewMock()
	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)), WithConfig(cfg))
	defer checkClose(t, a)

	// Here we tell fn-test-utils to read file /proc/mounts and create a /tmp/salsa of 4MB
	bodOne := `{"readFile":"/proc/mounts", "createFile":"/tmp/salsa", "createFileSize": 4194304, "isDebug": true}`

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(bodOne)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	ctx := req.Context()

	var out bytes.Buffer
	callI, err := a.GetCall(ctx, FromRouteEvent(app, route, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(ctx, callI)
	if err != nil {
		t.Error("submit should not error:", err)
	}

	// we're using http format so this will have written a whole http request
	res, err := http.ReadResponse(bufio.NewReader(&out), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	// Let's fetch read output and write results. See fn-test-utils AppResponse struct (data field)
	var resp struct {
		R struct {
			MountsRead string `json:"/proc/mounts.read_output"`
			CreateFile string `json:"/tmp/salsa.create_error"`
		} `json:"data"`
	}

	json.NewDecoder(res.Body).Decode(&resp)

	// Let's check what mounts are on...
	mounts := strings.Split(resp.R.MountsRead, "\n")
	isFound := false
	isRootFound := false
	for _, mnt := range mounts {
		tokens := strings.Split(mnt, " ")
		if len(tokens) < 3 {
			continue
		}

		point := tokens[1]
		opts := tokens[3]

		// rw tmp dir with size and inode limits applied.
		if point == "/tmp" && opts == "rw,nosuid,nodev,noexec,relatime,size=1024k,nr_inodes=1024" {
			// good
			isFound = true
		} else if point == "/" && strings.HasPrefix(opts, "ro,") {
			// Read-only root, good...
			isRootFound = true
		}
	}

	if !isFound || !isRootFound {
		t.Fatal(`didn't get proper mounts for  /tmp or /, got /proc/mounts content of:\n`, resp.R.MountsRead)
	}

	// write file should have failed...
	if !strings.Contains(resp.R.CreateFile, "no space left on device") {
		t.Fatal(`limited tmpfs should generate fs full error, but got output: `, resp.R.CreateFile)
	}
}

// return a model with all fields filled in with fnproject/fn-test-utils:latest image, change as needed
func testCall() *models.Call {
	appName := "myapp"
	path := "/"
	image := "fnproject/fn-test-utils:latest"
	app := &models.App{ID: "app_id", Name: appName}

	const timeout = 10
	const idleTimeout = 20
	const memory = 256
	CPUs := models.MilliCPUs(200)
	method := "GET"
	url := "http://127.0.0.1:8080/r/" + appName + path
	payload := "payload"
	typ := "sync"
	format := "http"
	contentType := "suberb_type"
	contentLength := strconv.FormatInt(int64(len(payload)), 10)
	config := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": appName,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_CPUS":     CPUs.String(),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
		"DOUBLE_VAR":  "BIZ, BAZ",
	}
	headers := map[string][]string{
		// FromRouteEvent would insert these from original HTTP request
		"Content-Type":   []string{contentType},
		"Content-Length": []string{contentLength},
	}

	return &models.Call{
		AppID:       app.ID,
		Config:      config,
		Headers:     headers,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		CPUs:        CPUs,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}
}

type delayReader struct {
	once  sync.Once
	delay time.Duration

	io.Reader
}

func (r *delayReader) Read(b []byte) (int, error) {
	r.once.Do(func() { time.Sleep(r.delay) })
	return r.Reader.Read(b)
}

func TestPipesDontMakeSpuriousCalls(t *testing.T) {
	// if we swap out the pipes between tasks really fast, we need to ensure that
	// there are no spurious reads on the container's input that give us a bad
	// task output (i.e. 2nd task should succeed). if this test is fussing up,
	// make sure input swapping out is not racing, it is very likely not the test
	// that is finicky since this is a totally normal happy path (run 2 hot tasks
	// in the same container in a row).

	ctx := context.Background()
	call := testCall()
	call.Type = "sync"
	call.Format = "http"
	call.IdleTimeout = 60 // keep this bad boy alive
	call.Timeout = 4      // short
	app := &models.App{Name: "myapp"}

	app.ID = call.AppID

	route := &models.Route{
		Path:        call.Path,
		AppID:       call.AppID,
		Image:       call.Image,
		Type:        call.Type,
		Format:      call.Format,
		Timeout:     call.Timeout,
		IdleTimeout: call.IdleTimeout,
		Memory:      call.Memory,
	}

	ls := logs.NewMock()
	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)))
	defer checkClose(t, a)

	bodOne := `{"echoContent":"yodawg"}`
	req, err := http.NewRequest("GET", call.URL, strings.NewReader(bodOne))
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var outOne bytes.Buffer
	callI, err := a.GetCall(ctx, FromRouteEvent(app, route, req), WithWriter(&outOne))
	if err != nil {
		t.Fatal(err)
	}

	// this will time out after 4s, our reader reads after 5s
	t.Log("before submit one:", time.Now())
	err = a.Submit(ctx, callI)
	t.Log("after submit one:", time.Now())
	if err != nil {
		t.Error("got error from submit when task should succeed", err)
	}

	// if we submit the same ca to the hot container again,
	// this can be finicky if the
	// hot logic simply fails to re-use a container then this will
	// 'just work' but at one point this failed.

	bodTwo := `{"echoContent":"NODAWG"}`
	req, err = http.NewRequest("GET", call.URL, strings.NewReader(bodTwo))
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var outTwo bytes.Buffer
	callI, err = a.GetCall(ctx, FromRouteEvent(app, route, req), WithWriter(&outTwo))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("before submit two:", time.Now())
	err = a.Submit(ctx, callI)
	t.Log("after submit two:", time.Now())
	if err != nil {
		// don't do a Fatal so that we can read the body to see what really happened
		t.Error("got error from submit when task should succeed", err)
	}

	// we're using http format so this will have written a whole http request
	res, err := http.ReadResponse(bufio.NewReader(&outTwo), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	// {"request":{"echoContent":"yodawg"}}
	var resp struct {
		R struct {
			Body string `json:"echoContent"`
		} `json:"request"`
	}

	json.NewDecoder(res.Body).Decode(&resp)

	if resp.R.Body != "NODAWG" {
		t.Fatalf("body from second call was not what we wanted. boo. got wrong body: %v wanted: %v", resp.R.Body, "NODAWG")
	}
}

func TestNBIOResourceTracker(t *testing.T) {

	call := testCall()
	call.Type = "sync"
	call.Format = "http"
	call.IdleTimeout = 60
	call.Timeout = 30
	call.Memory = 50
	app := &models.App{ID: "app_id", Name: "myapp"}

	app.ID = call.AppID

	route := &models.Route{
		Path:        call.Path,
		AppID:       call.AppID,
		Image:       call.Image,
		Type:        call.Type,
		Format:      call.Format,
		Timeout:     call.Timeout,
		IdleTimeout: call.IdleTimeout,
		Memory:      call.Memory,
	}

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("bad config %+v", cfg)
	}

	cfg.EnableNBResourceTracker = true
	cfg.MaxTotalMemory = 280 * 1024 * 1024
	cfg.HotPoll = 20 * time.Millisecond

	ls := logs.NewMock()
	a := New(NewDirectCallDataAccess(ls, new(mqs.Mock)), WithConfig(cfg))
	defer checkClose(t, a)

	reqCount := 20
	errors := make(chan error, reqCount)
	for i := 0; i < reqCount; i++ {
		go func(i int) {
			body := `{sleepTime": 10000, "isDebug": true}`
			req, err := http.NewRequest("GET", call.URL, strings.NewReader(body))
			if err != nil {
				t.Fatal("unexpected error building request", err)
			}

			var outOne bytes.Buffer
			callI, err := a.GetCall(req.Context(), FromRouteEvent(app, route, req), WithWriter(&outOne))
			if err != nil {
				t.Fatal(err)
			}

			err = a.Submit(req.Context(), callI)
			errors <- err
		}(i)
	}

	ok := 0
	for i := 0; i < reqCount; i++ {
		err := <-errors
		t.Logf("Got response %v", err)
		if err == nil {
			ok++
		} else if err == models.ErrCallTimeoutServerBusy {
		} else {
			t.Fatalf("Unexpected error %v", err)
		}
	}

	// BUG: in theory, we should get 5 success. But due to hot polling/signalling,
	// some requests may aggresively get 'too busy' since our req to slot relationship
	// is not 1-to-1.
	// This occurs in hot function request bursts (such as this test case).
	// And when these requests repetitively poll the hotLauncher and system is
	// likely to decide that a new container is needed (since many requests are waiting)
	// which results in extra 'too busy' responses.
	//
	//
	// 280MB total ram with 50MB functions... 5 should succeed, rest should
	// get too busy
	if ok < 4 || ok > 5 {
		t.Fatalf("Expected successes, but got %d", ok)
	}
}
