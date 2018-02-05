package agent

import (
	"bufio"
	"bytes"
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

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
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

	app := &models.App{Name: appName, Config: cfg}
	app.SetDefaults()
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{
				AppID:       app.ID,
				Config:      rCfg,
				Path:        path,
				Image:       image,
				Type:        typ,
				Format:      format,
				Timeout:     timeout,
				IdleTimeout: idleTimeout,
				Memory:      memory,
			},
		}, nil,
	)

	a := New(NewDirectDataAccess(ds, ds, new(mqs.Mock)))
	defer a.Close()

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

	call, err := a.GetCall(
		WithWriter(w), // XXX (reed): order matters [for now]
		FromRequest(app.Name, path, req),
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
	if model.Payload != "" { // NOTE: this is expected atm
		t.Fatal("GetCall FromRequest should not fill payload, got non-empty payload", model.Payload)
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
	app.SetDefaults()
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

	cm := &models.Call{
		Config:      cfg,
		AppID:       app.ID,
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
	ds := datastore.NewMockInit(nil, nil, nil)

	a := New(NewDirectDataAccess(ds, ds, new(mqs.Mock)))
	defer a.Close()

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

func TestAsyncCallHeaders(t *testing.T) {
	app := &models.App{Name: "myapp"}
	app.SetDefaults()
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
		// FromRequest would insert these from original HTTP request
		"Content-Type":   {contentType},
		"Content-Length": {contentLength},
	}

	cm := &models.Call{
		Config:      config,
		Headers:     headers,
		AppID:       app.ID,
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
	ds := datastore.NewMockInit(nil, nil, nil)

	a := New(NewDirectDataAccess(ds, ds, new(mqs.Mock)))
	defer a.Close()

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

	loggyloo := logrus.WithFields(logrus.Fields{"yodawg": true})
	logger := setupLogger(loggyloo, 1*1024*1024)

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

	loggyloo := logrus.WithFields(logrus.Fields{"yodawg": true})
	logger := setupLogger(loggyloo, 10)

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

func TestSubmitError(t *testing.T) {
	app := &models.App{Name: "myapp"}
	app.SetDefaults()
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
		Config:      config,
		AppID:       app.ID,
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
	ds := datastore.NewMockInit(nil, nil, nil)

	a := New(NewDirectDataAccess(ds, ds, new(mqs.Mock)))
	defer a.Close()

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	err = a.Submit(callI)
	if err == nil {
		t.Fatal("expected error but got none")
	}

	if cm.Status != "error" {
		t.Fatal("expected status to be set to 'error' but was", cm.Status)
	}

	if cm.Error == "" {
		t.Fatal("expected error string to be set on call")
	}
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

	app := &models.App{Name: appName}
	app.SetDefaults()
	// we need to load in app & route so that FromRequest works
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{
				Path:        path,
				AppID:       app.ID,
				Image:       "fnproject/fn-test-utils",
				Type:        "sync",
				Format:      "http", // this _is_ the test
				Timeout:     5,
				IdleTimeout: 10,
				Memory:      128,
			},
		}, nil,
	)

	a := New(NewDirectDataAccess(ds, ds, new(mqs.Mock)))
	defer a.Close()

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
	callI, err := a.GetCall(FromRequest(app.Name, path, req), WithWriter(&out))
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
		Path:        "/yoyo",
		Image:       "fnproject/fn-test-utils",
		Type:        "sync",
		Format:      "http",
		Timeout:     1,
		IdleTimeout: 2,
		Memory:      math.MaxUint64,
	}

	// FromModel doesn't need a datastore, for now...
	ds := datastore.NewMockInit(nil, nil, nil)

	a := New(NewCachedDataAccess(NewDirectDataAccess(ds, ds, new(mqs.Mock))))
	defer a.Close()

	_, err := a.GetCall(FromModel(call))
	if err != models.ErrCallTimeoutServerBusy {
		t.Fatal("did not get expected err, got: ", err)
	}
}

// return a model with all fields filled in with fnproject/fn-test-utils:latest image, change as needed
func testCall() *models.Call {
	appName := "myapp"
	path := "/"
	image := "fnproject/fn-test-utils:latest"
	app := &models.App{Name: appName}
	app.SetDefaults()
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
		// FromRequest would insert these from original HTTP request
		"Content-Type":   []string{contentType},
		"Content-Length": []string{contentLength},
	}

	return &models.Call{
		Config:      config,
		Headers:     headers,
		AppID:       app.ID,
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
	ca.Format = "http"
	ca.IdleTimeout = 60 // keep this bad boy alive
	ca.Timeout = 4      // short
	app := &models.App{Name: "myapp"}
	app.SetDefaults()
	app.ID = ca.AppID
	// we need to load in app & route so that FromRequest works
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{
				Path:        ca.Path,
				AppID:       ca.ID,
				Image:       ca.Image,
				Type:        ca.Type,
				Format:      ca.Format,
				Timeout:     ca.Timeout,
				IdleTimeout: ca.IdleTimeout,
				Memory:      ca.Memory,
			},
		}, nil,
	)

	a := New(NewDirectDataAccess(ds, ds, new(mqs.Mock)))
	defer a.Close()

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
	callI, err := a.GetCall(FromRequest(app.Name, ca.Path, req), WithWriter(&outOne))
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
	callI, err = a.GetCall(FromRequest(app.Name, ca.Path, req), WithWriter(&outTwo))
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

func TestPipesDontMakeSpuriousCalls(t *testing.T) {
	// if we swap out the pipes between tasks really fast, we need to ensure that
	// there are no spurious reads on the container's input that give us a bad
	// task output (i.e. 2nd task should succeed). if this test is fussing up,
	// make sure input swapping out is not racing, it is very likely not the test
	// that is finicky since this is a totally normal happy path (run 2 hot tasks
	// in the same container in a row).

	call := testCall()
	call.Type = "sync"
	call.Format = "http"
	call.IdleTimeout = 60 // keep this bad boy alive
	call.Timeout = 4      // short
	app := &models.App{Name: "myapp"}
	app.SetDefaults()
	app.ID = call.AppID
	// we need to load in app & route so that FromRequest works
	ds := datastore.NewMockInit(
		[]*models.App{app},
		[]*models.Route{
			{
				Path:        call.Path,
				AppID:       call.AppID,
				Image:       call.Image,
				Type:        call.Type,
				Format:      call.Format,
				Timeout:     call.Timeout,
				IdleTimeout: call.IdleTimeout,
				Memory:      call.Memory,
			},
		}, nil,
	)

	a := New(NewDirectDataAccess(ds, ds, new(mqs.Mock)))
	defer a.Close()

	bodOne := `{"echoContent":"yodawg"}`
	req, err := http.NewRequest("GET", call.URL, strings.NewReader(bodOne))
	if err != nil {
		t.Fatal("unexpected error building request", err)
	}

	var outOne bytes.Buffer
	callI, err := a.GetCall(FromRequest(app.Name, call.Path, req), WithWriter(&outOne))
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
	callI, err = a.GetCall(FromRequest(app.Name, call.Path, req), WithWriter(&outTwo))
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
