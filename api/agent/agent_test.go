package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	_ "github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
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
	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	typ := "sync"

	cfg := models.Config{"APP_VAR": "FOO"}
	rCfg := models.Config{"FN_VAR": "BAR"}

	app := &models.App{ID: "app_id", Config: cfg}
	fn := &models.Fn{
		ID:     "fn_id",
		AppID:  app.ID,
		Config: rCfg,
		Image:  image,
		ResourceConfig: models.ResourceConfig{Timeout: timeout,
			IdleTimeout: idleTimeout,
			Memory:      memory,
		},
	}

	a := New()
	defer checkClose(t, a)

	w := httptest.NewRecorder()

	method := "GET"
	url := "http://127.0.0.1:8080/invoke/" + fn.ID
	payload := "payload"
	contentLength := strconv.Itoa(len(payload))
	req, err := http.NewRequest(method, url, strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("MYREALHEADER", "FOOLORD")
	req.Header.Add("MYREALHEADER", "FOOPEASANT")
	req.Header.Add("Content-Length", contentLength)
	req.Header.Add("FN_PATH", "thewrongfn") // ensures that this doesn't leak out, should be overwritten

	call, err := a.GetCall(
		WithWriter(w), // XXX (reed): order matters [for now]
		FromHTTPFnRequest(app, fn, req),
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
	if model.Image != image {
		t.Fatal("image mismatch", model.Image, image)
	}
	if model.Type != "sync" {
		t.Fatal("fn type mismatch", model.Type)
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
	if model.Payload != "" { // NOTE: this is expected atm
		t.Fatal("GetCall FromRequest should not fill payload, got non-empty payload", model.Payload)
	}

	expectedConfig := map[string]string{
		"FN_MEMORY": strconv.Itoa(memory),
		"FN_TYPE":   typ,
		"APP_VAR":   "FOO",
		"FN_VAR":    "BAR",
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

	// TODO check response writer for fn headers
}

func TestCallConfigurationModel(t *testing.T) {
	fn := &models.Fn{ID: "fn_id"}

	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	method := "GET"
	url := "http://127.0.0.1:8080/invoke/" + fn.ID
	payload := "payload"
	typ := "sync"
	cfg := models.Config{
		"FN_MEMORY": strconv.Itoa(memory),
		"FN_TYPE":   typ,
		"APP_VAR":   "FOO",
		"FN_VAR":    "BAR",
	}

	cm := &models.Call{
		FnID:        fn.ID,
		Config:      cfg,
		Image:       image,
		Type:        typ,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	a := New()
	defer checkClose(t, a)

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	req := callI.(*call).req

	var b bytes.Buffer
	io.Copy(&b, req.Body)

	if b.String() != payload {
		t.Fatal("expected payload to match, but it was a lie")
	}
}

func TestGetCallFromModelRoundTripACall(t *testing.T) {
	payload := "payload"
	contentType := "suberb_type"
	contentLength := strconv.FormatInt(int64(len(payload)), 10)
	headers := map[string][]string{
		// FromRequest would insert these from original HTTP request
		"Content-Type":   {contentType},
		"Content-Length": {contentLength},
	}

	cm := &models.Call{
		FnID:    "fn_id",
		Headers: headers,
		Payload: payload,
	}

	a := New()
	defer checkClose(t, a)

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	// make sure headers seem reasonable
	req := callI.(*call).req

	// These should be here based on payload length and/or fn_header_* original headers
	expectedHeaders := make(http.Header)
	expectedHeaders.Set("Content-Type", contentType)
	expectedHeaders.Set("Content-Length", strconv.FormatInt(int64(len(payload)), 10))

	checkExpectedHeaders(t, expectedHeaders, req.Header)

	var b bytes.Buffer
	io.Copy(&b, req.Body)

	if b.String() != payload {
		t.Fatal("expected payload to match, but it was a lie")
	}
}

func TestLoggerIsStringerAndWorks(t *testing.T) {
	// TODO test limit writer, logrus writer, etc etc

	var call models.Call
	logger := setupLogger(context.Background(), 1*1024*1024, true, &call)

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
	logger := setupLogger(context.Background(), 10, true, &call)

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
	app := &models.App{ID: "app_id", Name: "myapp"}
	fn := &models.Fn{ID: "fn_id", AppID: app.ID}

	image := "fnproject/fn-test-utils"
	const timeout = 10
	const idleTimeout = 20
	const memory = 256
	method := "GET"
	url := "http://127.0.0.1:8080/invoke/" + fn.ID
	payload := `{"sleepTime": 0, "isDebug": true, "isCrash": true}`
	typ := "sync"
	config := map[string]string{
		"FN_LISTENER": "unix:" + filepath.Join(iofsDockerMountDest, udsFilename),
		"FN_APP_ID":   app.ID,
		"FN_FN_ID":    fn.ID,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"FN_VAR":      "BAR",
		"DOUBLE_VAR":  "BIZ, BAZ",
	}

	cm := &models.Call{
		AppID:       app.ID,
		FnID:        fn.ID,
		Config:      config,
		Image:       image,
		Type:        typ,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	a := New()
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

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
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

func TestHungFDK(t *testing.T) {
	app := &models.App{ID: "app_id"}
	fn := &models.Fn{
		ID:     "fn_id",
		Image:  "fnproject/fn-test-utils",
		Config: models.Config{"ENABLE_INIT_DELAY_MSEC": "5000"},
		ResourceConfig: models.ResourceConfig{
			Timeout:     5,
			IdleTimeout: 10,
			Memory:      128,
		},
	}

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	cfg, err := NewConfig()
	cfg.HotStartTimeout = time.Duration(3) * time.Second
	a := New(WithConfig(cfg))
	defer checkClose(t, a)

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(`{}`)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err == nil {
		t.Fatal("submit should error!")
	}
	if err != models.ErrContainerInitTimeout {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestDockerPullHungRepo(t *testing.T) {
	hung, cancel := context.WithCancel(context.Background())
	garbageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// version check seem to have a sane timeout in docker, let's serve this, then stop
		if r.URL.String() == "/v2/" {
			w.WriteHeader(200)
			return
		}
		<-hung.Done()
	}))
	defer garbageServer.Close()
	defer cancel()

	dest := strings.TrimPrefix(garbageServer.URL, "http://")

	app := &models.App{ID: "app_id"}
	fn := &models.Fn{
		ID:    "fn_id",
		Image: dest + "/fnproject/fn-test-utils",
		ResourceConfig: models.ResourceConfig{
			Timeout:     5,
			IdleTimeout: 10,
			Memory:      128,
		},
	}

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	cfg, err := NewConfig()
	cfg.HotPullTimeout = time.Duration(5) * time.Second
	a := New(WithConfig(cfg))
	defer checkClose(t, a)

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(`{}`)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err == nil {
		t.Fatal("submit should error!")
	}
	if err != models.ErrDockerPullTimeout {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestDockerPullUnAuthorizedRepo(t *testing.T) {
	garbageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// version check seem to have a sane timeout in docker, let's serve this, then stop
		if r.URL.String() == "/v2/" {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(401)
		return
	}))
	defer garbageServer.Close()

	dest := strings.TrimPrefix(garbageServer.URL, "http://")

	app := &models.App{ID: "app_id"}
	fn := &models.Fn{
		ID:    "fn_id",
		Image: dest + "/fnproject/fn-test-utils",
		ResourceConfig: models.ResourceConfig{
			Timeout:     5,
			IdleTimeout: 10,
			Memory:      128,
		},
	}

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	cfg, err := NewConfig()
	cfg.HotPullTimeout = time.Duration(5) * time.Second
	a := New(WithConfig(cfg))
	defer checkClose(t, a)

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(`{}`)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err == nil {
		t.Fatal("submit should error!")
	}
	if models.GetAPIErrorCode(err) != http.StatusBadGateway {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestDockerPullBadRepo(t *testing.T) {

	garbageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "enjoy this lovely garbage")
	}))
	defer garbageServer.Close()

	dest := strings.TrimPrefix(garbageServer.URL, "http://")

	app := &models.App{ID: "app_id"}
	fn := &models.Fn{
		ID:    "fn_id",
		Image: dest + "/fnproject/fn-test-utils",
		ResourceConfig: models.ResourceConfig{
			Timeout:     5,
			IdleTimeout: 10,
			Memory:      128,
		},
	}

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	cfg, err := NewConfig()
	a := New(WithConfig(cfg))
	defer checkClose(t, a)

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(`{}`)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err == nil {
		t.Fatal("submit should error!")
	}
	if !models.IsAPIError(err) || !strings.HasPrefix(err.Error(), "Failed to pull image ") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestHTTPWithoutContentLengthWorks(t *testing.T) {
	app := &models.App{ID: "app_id"}
	fn := &models.Fn{
		ID:    "fn_id",
		Image: "fnproject/fn-test-utils",
		ResourceConfig: models.ResourceConfig{
			Timeout:     5,
			IdleTimeout: 10,
			Memory:      128,
		},
	}

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	a := New()
	defer checkClose(t, a)

	bodOne := `{"echoContent":"yodawg"}`

	// get a req that uses the dummy reader, so that this can't determine
	// the size of the body and set content length (user requests may also
	// forget to do this, and we _should_ read it as chunked without issue).
	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(bodOne)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	// grab a buffer so we can read what gets written to this guy
	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
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
		FnID:        id.New().String(),
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Timeout:     1,
		IdleTimeout: 2,
		Memory:      math.MaxUint64,
	}

	a := New()
	defer checkClose(t, a)

	_, err := a.GetCall(FromModel(call))
	if err != models.ErrCallResourceTooBig {
		t.Fatal("did not get expected err, got: ", err)
	}
}

//
// Tmp directory should be RW by default.
//
func TestTmpFsRW(t *testing.T) {

	app := &models.App{ID: "app_id"}

	fn := &models.Fn{
		ID:    "fn_id",
		AppID: app.ID,
		Image: "fnproject/fn-test-utils",
		ResourceConfig: models.ResourceConfig{Timeout: 5,
			IdleTimeout: 10,
			Memory:      128,
		},
	}

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	a := New()
	defer checkClose(t, a)

	// Here we tell fn-test-utils to read file /proc/mounts and create a /tmp/salsa of 4MB
	bodOne := `{"readFile":"/proc/mounts", "createFile":"/tmp/salsa", "createFileSize": 4194304, "isDebug": true}`

	req, err := http.NewRequest("GET", url, &dummyReader{Reader: strings.NewReader(bodOne)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
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
	app := &models.App{ID: "app_id", Name: appName}

	fn := &models.Fn{
		ID:    "fn_id",
		AppID: app.ID,
		Image: "fnproject/fn-test-utils",
		ResourceConfig: models.ResourceConfig{Timeout: 5,
			IdleTimeout: 10,
			Memory:      64,
		},
	}
	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	cfg, err := NewConfig()
	if err != nil {
		t.Fatal(err)
	}

	cfg.MaxTmpFsInodes = 1025

	a := New(WithConfig(cfg))
	defer checkClose(t, a)

	// Here we tell fn-test-utils to read file /proc/mounts and create a /tmp/salsa of 4MB
	bodOne := `{"readFile":"/proc/mounts", "createFile":"/tmp/salsa", "createFileSize": 4194304, "isDebug": true}`

	req, err := http.NewRequest("POST", url, &dummyReader{Reader: strings.NewReader(bodOne)})
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var out bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))

	callI.Model().TmpFsSize = 1
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
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
		if point == "/tmp" && opts == "rw,nosuid,nodev,noexec,relatime,size=1024k,nr_inodes=1025" {
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
	image := "fnproject/fn-test-utils:latest"
	fn := &models.Fn{ID: "fn_id"}

	const timeout = 10
	const idleTimeout = 20
	const memory = 256
	method := "GET"
	url := "http://127.0.0.1:8080/invoke/" + fn.ID
	payload := "payload"
	typ := "sync"
	contentType := "suberb_type"
	contentLength := strconv.FormatInt(int64(len(payload)), 10)
	config := map[string]string{
		"FN_MEMORY":  strconv.Itoa(memory),
		"FN_TYPE":    typ,
		"APP_VAR":    "FOO",
		"FN_VAR":     "BAR",
		"DOUBLE_VAR": "BIZ, BAZ",
	}
	headers := map[string][]string{
		// FromRequest would insert these from original HTTP request
		"Content-Type":   []string{contentType},
		"Content-Length": []string{contentLength},
	}

	return &models.Call{
		FnID:        fn.ID,
		Config:      config,
		Headers:     headers,
		Image:       image,
		Type:        typ,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}
}

func TestPipesAreClear(t *testing.T) {
	// The basic idea here is to make a call start a hot container, and the
	// first call has a reader that only reads after a delay, which is beyond
	// the boundary of the first call's timeout. Then, run a second call
	// with a different body that also has a slight delay. make sure the second
	// call gets the correct body.  This ensures the input paths for calls do not
	// overlap into the same container and they don't block past timeout.
	// TODO make sure the second call does not get the first call's body if
	// we write the first call's body in before the second's (it was tested
	// but not put in stone here, the code is ~same).
	//
	// causal (seconds):
	// T1=start task one, T1TO=task one times out, T2=start task two
	// T1W=task one writes, T2W=task two writes
	//
	//
	//  1s  2   3    4   5   6
	// ---------------------------
	//
	// T1-------T1TO-T2-T1W--T2W--

	ca := testCall()
	ca.Type = "sync"
	ca.IdleTimeout = 60 // keep this bad boy alive
	ca.Timeout = 4      // short
	app := &models.App{ID: ca.AppID}

	fn := &models.Fn{
		AppID: ca.AppID,
		ID:    ca.FnID,
		Image: ca.Image,
		ResourceConfig: models.ResourceConfig{
			Timeout:     ca.Timeout,
			IdleTimeout: ca.IdleTimeout,
			Memory:      ca.Memory,
		},
	}

	a := New()
	defer checkClose(t, a)

	// test read this body after 5s (after call times out) and make sure we don't get yodawg
	// TODO could read after 10 seconds, to make sure the 2nd task's input stream isn't blocked
	// TODO we need to test broken HTTP output from a task should return a useful error
	bodOne := `{"echoContent":"yodawg"}`
	delayBodyOne := &delayReader{Reader: strings.NewReader(bodOne), delay: 5 * time.Second}

	req, err := http.NewRequest("GET", ca.URL, delayBodyOne)
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}
	// NOTE: using chunked here seems to perplex the go http request reading code, so for
	// the purposes of this test, set this. json also works.
	req.ContentLength = int64(len(bodOne))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodOne)))

	var outOne bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&outOne))
	if err != nil {
		t.Fatal(err)
	}

	// this will time out after 4s, our reader reads after 5s
	t.Log("before submit one:", time.Now())
	err = a.Submit(callI)
	t.Log("after submit one:", time.Now())
	if err == nil {
		t.Error("expected error but got none")
	}
	t.Log("first guy err:", err)

	if len(outOne.String()) > 0 {
		t.Fatal("input should not have been read, producing 0 output, got:", outOne.String())
	}

	// if we submit another call to the hot container, this can be finicky if the
	// hot logic simply fails to re-use a container then this will 'just work'
	// but at one point this failed.

	// only delay this body 2 seconds, so that we read at 6s (first writes at 5s) before time out
	bodTwo := `{"echoContent":"NODAWG"}`
	delayBodyTwo := &delayReader{Reader: strings.NewReader(bodTwo), delay: 2 * time.Second}

	req, err = http.NewRequest("GET", ca.URL, delayBodyTwo)
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}
	req.ContentLength = int64(len(bodTwo))
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodTwo)))

	var outTwo bytes.Buffer
	callI, err = a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&outTwo))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("before submit two:", time.Now())
	err = a.Submit(callI)
	t.Log("after submit two:", time.Now())
	if err != nil {
		t.Error("got error from submit when task should succeed", err)
	}

	body := outTwo.String()

	// we're using http format so this will have written a whole http request
	res, err := http.ReadResponse(bufio.NewReader(&outTwo), nil)
	if err != nil {
		t.Fatalf("error reading body. err: %v body: %s", err, body)
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

	// NOTE: we need to make sure that 2 containers didn't launch to process
	// this. this isn't perfect but we really should be able to run 2 tasks
	// sequentially even if the first times out in the same container, so this
	// ends up testing hot container management more than anything. i do not like
	// digging around in the concrete type of ca for state stats but this seems
	// the best way to ensure 2 containers aren't launched. this does have the
	// shortcoming that if the first container dies and another launches, we
	// don't see it and this passes when it should not.  feel free to amend...
	callConcrete := callI.(*call)
	var count uint64
	for _, up := range callConcrete.slots.getStats().containerStates {
		up += count
	}
	if count > 1 {
		t.Fatalf("multiple containers launched to service this test. this shouldn't be. %d", count)
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

func TestCallsDontInterlace(t *testing.T) {
	// this runs a task that times out and then writes bytes after the timeout,
	// and then runs another task before those bytes are written. the 2nd task
	// should be successful using the same container, and the 1st task should
	// time out and its bytes shouldn't interfere with the 2nd task (this should
	// be a totally normal happy path).

	call := testCall()
	call.Type = "sync"
	call.IdleTimeout = 60 // keep this bad boy alive
	call.Timeout = 4      // short
	app := &models.App{Name: "myapp"}

	app.ID = call.AppID

	fn := &models.Fn{
		ID:    "fn_id",
		AppID: call.AppID,
		Image: call.Image,
		ResourceConfig: models.ResourceConfig{
			Timeout:     call.Timeout,
			IdleTimeout: call.IdleTimeout,
			Memory:      call.Memory,
		},
	}

	a := New()
	defer checkClose(t, a)

	bodOne := `{"echoContent":"yodawg"}`
	req, err := http.NewRequest("GET", call.URL, strings.NewReader(bodOne))
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var outOne bytes.Buffer
	callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&outOne))
	if err != nil {
		t.Fatal(err)
	}

	// this will time out after 4s, our reader reads after 5s
	t.Log("before submit one:", time.Now())
	err = a.Submit(callI)
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
	callI, err = a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&outTwo))
	if err != nil {
		t.Fatal(err)
	}

	t.Log("before submit two:", time.Now())
	err = a.Submit(callI)
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
	call.IdleTimeout = 60
	call.Timeout = 30
	call.Memory = 50
	app := &models.App{ID: "app_id", Name: "myapp"}

	app.ID = call.AppID

	fn := &models.Fn{
		ID:    call.FnID,
		AppID: call.AppID,
		Image: call.Image,
		ResourceConfig: models.ResourceConfig{
			Timeout:     call.Timeout,
			IdleTimeout: call.IdleTimeout,
			Memory:      call.Memory,
		},
	}

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("bad config %+v", cfg)
	}

	cfg.EnableNBResourceTracker = true
	cfg.MaxTotalMemory = 280 * 1024 * 1024
	cfg.HotPoll = 20 * time.Millisecond

	a := New(WithConfig(cfg))
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
			callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&outOne))
			if err != nil {
				t.Fatal(err)
			}

			err = a.Submit(callI)
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

func TestDockerAuthExtn(t *testing.T) {
	modelCall := &models.Call{
		AppID:       id.New().String(),
		FnID:        id.New().String(),
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Timeout:     1,
		IdleTimeout: 2,
	}
	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("bad config %+v", cfg)
	}

	a := New()
	defer checkClose(t, a)

	callIf, err := a.GetCall(FromModel(modelCall))
	if err != nil {
		t.Fatal(err)
	}
	call := callIf.(*call)

	ctx := context.TODO()

	errC := make(chan error, 10)

	c := newHotContainer(ctx, nil, nil, call, cfg, id.New().String(), "", errC)
	if c == nil {
		err := <-errC
		t.Fatal("got unexpected err: ", err)
	}
	da, err := c.DockerAuth(ctx, modelCall.Image)
	if da != nil {
		t.Fatal("invalid docker auth configuration")
	}
	if err != nil {
		t.Fatal("got unexpected err: ", err)
	}

	c = newHotContainer(ctx, nil, nil, call, cfg, id.New().String(), "TestRegistryToken", errC)
	if c == nil {
		err := <-errC
		t.Fatal("got unexpected err: ", err)
	}
	da, err = c.DockerAuth(ctx, modelCall.Image)
	if da == nil {
		t.Fatal("invalid docker auth configuration")
	}
	if da.RegistryToken != "TestRegistryToken" {
		t.Fatalf("unexpected registry token %s", da.RegistryToken)
	}
}

func TestCheckSocketDestination(t *testing.T) {
	tmpDir, err := ioutil.TempDir(os.TempDir(), "testSocketPerms")
	if err != nil {
		t.Fatal("failed to create temp tmpDir", err)
	}
	defer os.RemoveAll(tmpDir)

	goodSock := filepath.Join(tmpDir, "fn.sock")
	s, err := net.Listen("unix", goodSock)
	if err != nil {
		t.Fatal("failed to create socket", err)
	}
	defer s.Close()

	err = os.Chmod(goodSock, 0666)
	if err != nil {
		t.Fatal("failed to change perms", err)
	}
	notASocket := filepath.Join(tmpDir, "notasock.sock")

	err = ioutil.WriteFile(notASocket, []byte{0}, 0666)
	if err != nil {
		t.Fatalf("Failed to create empty sock")
	}

	goodSymlink := filepath.Join(tmpDir, "goodlink.sock")
	err = os.Symlink("fn.sock", goodSymlink)
	if err != nil {
		t.Fatalf("Failed to create symlink")
	}

	badLinkNonExistant := filepath.Join(tmpDir, "badlinknonExist.sock")
	err = os.Symlink("noxexistatnt.sock", badLinkNonExistant)
	if err != nil {
		t.Fatalf("Failed to create symlink")
	}

	badLinkOutOfPath := filepath.Join(tmpDir, "badlinkoutofpath.sock")
	err = os.Symlink(filepath.Join("..", filepath.Base(tmpDir), "fn.sock"), badLinkOutOfPath)
	if err != nil {
		t.Fatalf("Failed to create symlink")
	}

	for _, good := range []string{goodSock, goodSymlink} {
		t.Run(filepath.Base(good), func(t *testing.T) {
			err := checkSocketDestination(good)
			if err != nil {
				t.Errorf("Expected no error got, %s", err)
			}
		})
	}
	for _, bad := range []string{notASocket, badLinkNonExistant, badLinkOutOfPath, filepath.Join(tmpDir, "notAFile"), tmpDir} {
		t.Run(filepath.Base(bad), func(t *testing.T) {
			err := checkSocketDestination(bad)
			if err == nil {
				t.Errorf("Expected an error but got none")
			}
		})
	}
}

func TestContainerDisableIO(t *testing.T) {
	modelCall := &models.Call{
		AppID:       id.New().String(),
		FnID:        id.New().String(),
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Timeout:     1,
		IdleTimeout: 2,
	}
	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("bad config %+v", cfg)
	}

	a := New()
	defer checkClose(t, a)

	// NOTE: right now we disable stdin by default so this test should pass.
	// if you're adding back stdin and this fails, that is why.
	// NOTE: specify noop as the logger, stdout will get sent to stderr,
	// and we should get back a noop writer from the container for both.
	callIf, err := a.GetCall(FromModel(modelCall),
		WithLogger(common.NoopReadWriteCloser{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	call := callIf.(*call)

	ctx := context.TODO()

	errC := make(chan error, 10)

	c := newHotContainer(ctx, nil, nil, call, cfg, id.New().String(), "", errC)
	if c == nil {
		err := <-errC
		t.Fatal("got unexpected err: ", err)
	}

	// we need to test that our concrete container type returns the noop
	// writers and readers to the docker driver (ie no decorators), which
	// the docker driver currently uses to disable stdin/stdout/stderr at
	// the container level (save some bytes)

	stdin := c.Input()
	stdout, stderr := c.Logger()

	_, stdinOff := stdin.(common.NoopReadWriteCloser)
	_, stdoutOff := stdout.(common.NoopReadWriteCloser)
	_, stderrOff := stderr.(common.NoopReadWriteCloser)

	if !stdinOff {
		t.Error("stdin is enabled, stdin should be disabled")
	}
	if !stdoutOff {
		t.Error("stdout is enabled, stdout should be disabled")
	}
	if !stderrOff {
		t.Error("stderr is enabled, stderr should be disabled")
	}
}

func TestSlotErrorRetention(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(10*time.Second))
	defer cancel()

	app := &models.App{ID: "app_id"}
	fn := &models.Fn{
		ID:     "fn_id",
		Image:  "fnproject/fn-test-utils",
		Config: models.Config{"ENABLE_INIT_DELAY_MSEC": "100"},
		ResourceConfig: models.ResourceConfig{
			Timeout:     5,
			IdleTimeout: 10,
			Memory:      128,
		},
	}

	url := "http://127.0.0.1:8080/invoke/" + fn.ID

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("error %v in agent config cfg=%+v", err, cfg)
	}
	cfg.EnableFDKDebugInfo = true

	a := New(WithConfig(cfg))

	defer checkClose(t, a)

	const echoContent = "parsley"
	const concurrency = 2

	var uniqId string

	checkBody := func(res *http.Response) {
		var resp struct {
			R struct {
				Body string `json:"echoContent"`
			} `json:"request"`
		}

		json.NewDecoder(res.Body).Decode(&resp)
		if resp.R.Body != echoContent {
			t.Fatalf(`didn't get a %s in the body: %s`, echoContent, resp.R.Body)
		}

		if res != nil && res.Body != nil {
			res.Body.Close()
		}
	}

	// just spawn a container
	{
		body := fmt.Sprintf(`{"sleepTime": 0, "echoContent":"%s"}`, echoContent)

		req, err := http.NewRequest("GET", url, strings.NewReader(body))
		if err != nil {
			t.Fatalf("unexpected error building request %v", err)
		}
		req = req.WithContext(ctx)

		var out bytes.Buffer
		callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
		if err != nil {
			t.Fatalf("unexpected error building call %v", err)
		}

		uniqId = callI.Model().ID
		callI.Model().Config["ENABLE_FAIL_IF_FN_SPAWN_CALL_ID_NONMATCH"] = uniqId
		callI.Model().Config["ENABLE_FAIL_IF_FN_SPAWN_CALL_ID_NONMATCH_MSEC"] = "100"

		err = a.Submit(callI)
		if err != nil {
			t.Fatalf("submit should not error %v", err)
		}

		res, err := http.ReadResponse(bufio.NewReader(&out), nil)
		if err != nil {
			t.Fatalf("read resp should not error %v", err)
		}

		checkBody(res)
	}

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for idx := 0; idx < concurrency; idx++ {
		go func(id int) {
			defer wg.Done()
			dctx, cancel := context.WithTimeout(ctx, time.Duration(1000*time.Millisecond))
			defer cancel()

			for dctx.Err() == nil {

				body := fmt.Sprintf(`{"sleepTime": 5, "echoContent":"%s"}`, echoContent)
				req, err := http.NewRequest("GET", url, strings.NewReader(body))
				if err != nil {
					t.Fatalf("unexpected error building request %v", err)
				}
				req = req.WithContext(ctx)

				var out bytes.Buffer
				callI, err := a.GetCall(FromHTTPFnRequest(app, fn, req), WithWriter(&out))
				if err != nil {
					t.Fatalf("unexpected error building call %v", err)
				}

				callI.Model().Config["ENABLE_FAIL_IF_FN_SPAWN_CALL_ID_NONMATCH"] = uniqId
				callI.Model().Config["ENABLE_FAIL_IF_FN_SPAWN_CALL_ID_NONMATCH_MSEC"] = "100"

				err = a.Submit(callI)
				if err != nil {
					t.Fatalf("submit should not error %v", err)
				}

				res, err := http.ReadResponse(bufio.NewReader(&out), nil)
				if err != nil {
					t.Fatalf("read resp should not error %v", err)
				}

				checkBody(res)
			}
		}(idx)
	}

	wg.Wait()
}

// Custom driver
type customDriver struct {
	drv drivers.Driver

	isClosed bool
	isBefore bool
	isAfter  bool

	beforeFn drivers.BeforeCall
	afterFn  drivers.AfterCall
	closeFn  func()
}

// implements Driver
func (d *customDriver) CreateCookie(ctx context.Context, task drivers.ContainerTask) (drivers.Cookie, error) {
	cookie, err := d.drv.CreateCookie(ctx, task)
	if err != nil {
		return cookie, err
	}

	task.WrapClose(func(closer func()) func() {
		return func() {
			closer()
			d.isClosed = true
			if d.closeFn != nil {
				d.closeFn()
			}
		}
	})

	task.WrapBeforeCall(func(before drivers.BeforeCall) drivers.BeforeCall {
		return func(ctx context.Context, call *models.Call, extn drivers.CallExtensions) error {
			err := before(ctx, call, extn)
			if err != nil {
				logrus.WithError(err).Fatal("expected no error but got")
				return err
			}
			d.isBefore = true
			if d.beforeFn != nil {
				return d.beforeFn(ctx, call, extn)
			}
			return nil
		}
	})

	task.WrapAfterCall(func(after drivers.AfterCall) drivers.AfterCall {
		return func(ctx context.Context, call *models.Call, extn drivers.CallExtensions) error {
			err := after(ctx, call, extn)
			if err != nil {
				logrus.WithError(err).Fatal("expected no error but got")
				return err
			}
			d.isAfter = true
			if d.afterFn != nil {
				return d.afterFn(ctx, call, extn)
			}
			return nil
		}
	})

	return cookie, nil
}

// implements Driver
func (d *customDriver) SetPullImageRetryPolicy(policy common.BackOffConfig, checker drivers.RetryErrorChecker) error {
	return d.drv.SetPullImageRetryPolicy(policy, checker)
}

// implements Driver
func (d *customDriver) Close() error {
	return d.drv.Close()
}

// implements Driver
func (d *customDriver) GetSlotKeyExtensions(extn map[string]string) string {
	return d.drv.GetSlotKeyExtensions(extn)
}

var _ drivers.Driver = &customDriver{}

func createModelCall(appId string) *models.Call {
	app := &models.App{ID: appId, Name: "myapp"}
	fn := &models.Fn{ID: "fn_id", AppID: app.ID}

	image := "fnproject/fn-test-utils"
	const timeout = 10
	const idleTimeout = 20
	const memory = 256
	method := "GET"
	url := "http://127.0.0.1:8080/invoke/" + fn.ID
	payload := `{isDebug": true}`
	typ := "sync"
	config := map[string]string{
		"FN_LISTENER": "unix:" + filepath.Join(iofsDockerMountDest, udsFilename),
		"FN_APP_ID":   app.ID,
		"FN_FN_ID":    fn.ID,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_TYPE":     typ,
	}

	cm := &models.Call{
		AppID:       app.ID,
		FnID:        fn.ID,
		Config:      config,
		Image:       image,
		Type:        typ,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}
	return cm
}

func TestContainerBeforeAfterWrapOK(t *testing.T) {
	cm := createModelCall("TestContainerBeforeAfterWrapOK")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatal(err)
	}

	drv, err := NewDockerDriver(cfg)
	if err != nil {
		t.Fatal(err)
	}

	cust := &customDriver{
		drv: drv,
	}

	opts := []Option{}
	opts = append(opts, WithConfig(cfg))
	opts = append(opts, WithDockerDriver(cust))

	a := New(opts...)
	defer checkClose(t, a)

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err != nil {
		t.Fatalf("not expected error but got %v", err)
	}

	<-time.After(time.Duration(1 * time.Second))
	assert.Equal(t, cust.isClosed, false)
	assert.Equal(t, cust.isBefore, true)
	assert.Equal(t, cust.isAfter, true)
}

func TestContainerBeforeWrapNotOK(t *testing.T) {
	cm := createModelCall("TestContainerBeforeWrapNotOK")

	specialErr := errors.New("foo")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatal(err)
	}

	drv, err := NewDockerDriver(cfg)
	if err != nil {
		t.Fatal(err)
	}

	cust := &customDriver{
		drv: drv,
		beforeFn: func(ctx context.Context, call *models.Call, extn drivers.CallExtensions) error {
			return specialErr
		},
	}

	opts := []Option{}
	opts = append(opts, WithConfig(cfg))
	opts = append(opts, WithDockerDriver(cust))

	a := New(opts...)
	defer checkClose(t, a)

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err != specialErr {
		t.Fatalf("expected special error but got %v", err)
	}

	<-time.After(time.Duration(1 * time.Second))
	assert.Equal(t, cust.isClosed, true)
	assert.Equal(t, cust.isBefore, true)
	assert.Equal(t, cust.isAfter, false)
}

func TestContainerAfterWrapNotOK(t *testing.T) {
	cm := createModelCall("TestContainerAfterWrapNotOK")

	specialErr := errors.New("foo")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatal(err)
	}

	drv, err := NewDockerDriver(cfg)
	if err != nil {
		t.Fatal(err)
	}

	cust := &customDriver{
		drv: drv,
		afterFn: func(ctx context.Context, call *models.Call, extn drivers.CallExtensions) error {
			return specialErr
		},
	}

	opts := []Option{}
	opts = append(opts, WithConfig(cfg))
	opts = append(opts, WithDockerDriver(cust))

	a := New(opts...)
	defer checkClose(t, a)

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err != specialErr {
		t.Fatalf("expected special error but got %v", err)
	}

	<-time.After(time.Duration(1 * time.Second))
	assert.Equal(t, cust.isClosed, true)
	assert.Equal(t, cust.isBefore, true)
	assert.Equal(t, cust.isAfter, true)
}
