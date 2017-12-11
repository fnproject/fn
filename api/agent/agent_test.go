package agent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/sirupsen/logrus"
)

func TestCallConfigurationRequest(t *testing.T) {
	appName := "myapp"
	path := "/sleeper"
	image := "fnproject/sleeper"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	typ := "sync"
	format := "default"

	cfg := models.Config{"APP_VAR": "FOO"}
	rCfg := models.Config{"ROUTE_VAR": "BAR"}

	ds := datastore.NewMockInit(
		[]*models.App{
			{Name: appName, Config: cfg},
		},
		[]*models.Route{
			{
				Config:      rCfg,
				Path:        path,
				AppName:     appName,
				Image:       image,
				Type:        typ,
				Format:      format,
				Timeout:     timeout,
				IdleTimeout: idleTimeout,
				Memory:      memory,
			},
		}, nil,
	)

	a := New(ds, ds, new(mqs.Mock), AgentTypeFull)
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

	// let's assume we got there params from the URL
	params := make(Params, 0, 2)
	params = append(params, Param{Key: "YOGURT", Value: "garlic"})
	params = append(params, Param{Key: "LEGUME", Value: "garbanzo"})

	call, err := a.GetCall(
		WithWriter(w), // XXX (reed): order matters [for now]
		FromRequest(appName, path, req, params),
	)
	if err != nil {
		t.Fatal(err)
	}

	model := call.Model()

	// make sure the values are all set correctly
	if model.ID == "" {
		t.Fatal("model does not have id, GetCall should assign id")
	}
	if model.AppName != appName {
		t.Fatal("app name mismatch", model.AppName, appName)
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

	expectedBase := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": appName,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
	}

	expectedEnv := make(map[string]string)
	for k, v := range expectedBase {
		expectedEnv[k] = v
	}

	for k, v := range expectedBase {
		if v2 := model.BaseEnv[k]; v2 != v {
			t.Fatal("base var mismatch", k, v, v2, model.BaseEnv)
		}
		delete(expectedBase, k)
	}

	if len(expectedBase) > 0 {
		t.Fatal("got extra vars in base env set, add me to tests ;)", expectedBase)
	}

	expectedEnv["FN_CALL_ID"] = model.ID
	expectedEnv["FN_METHOD"] = method
	expectedEnv["FN_REQUEST_URL"] = url

	// add expected parameters from URL
	for _, val := range params {
		expectedEnv[fmt.Sprintf("FN_PARAM_%s", val.Key)] = val.Value
	}

	// do this before the "real" headers get sucked in cuz they are formatted differently
	expectedHeaders := make(http.Header)
	for k, v := range expectedEnv {
		expectedHeaders.Add(k, v)
	}

	// from the request headers (look different in env than in req.Header, idk, up to user anger)
	// req headers down cases things
	expectedEnv["FN_HEADER_Myrealheader"] = "FOOLORD, FOOPEASANT"
	expectedEnv["FN_HEADER_Content_Length"] = contentLength

	for k, v := range expectedEnv {
		if v2 := model.EnvVars[k]; v2 != v {
			t.Fatal("env var mismatch", k, v, v2, model.EnvVars)
		}
		delete(expectedEnv, k)
	}

	if len(expectedEnv) > 0 {
		t.Fatal("got extra vars in base env set, add me to tests ;)", expectedBase)
	}

	expectedHeaders.Add("MYREALHEADER", "FOOLORD")
	expectedHeaders.Add("MYREALHEADER", "FOOPEASANT")
	expectedHeaders.Add("Content-Length", contentLength)

	for k, vs := range req.Header {
		for i, v := range expectedHeaders[k] {
			if i >= len(vs) || vs[i] != v {
				t.Fatal("header mismatch", k, vs)
			}
		}
		delete(expectedHeaders, k)
	}

	if len(expectedHeaders) > 0 {
		t.Fatal("got extra headers, bad")
	}

	if w.Header()["Fn_call_id"][0] != model.ID {
		t.Fatal("response writer should have the call id, or else")
	}

	// TODO check response writer for route headers

	// TODO idk what param even is or how to get them, but need to test those
	// TODO we should double check the things we're rewriting defaults of, like type, format, timeout, idle_timeout
}

func TestCallConfigurationModel(t *testing.T) {
	appName := "myapp"
	path := "/sleeper"
	image := "fnproject/sleeper"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	method := "GET"
	url := "http://127.0.0.1:8080/r/" + appName + path
	payload := "payload"
	typ := "sync"
	format := "default"
	env := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": appName,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
		"DOUBLE_VAR":  "BIZ, BAZ",
	}

	cm := &models.Call{
		BaseEnv:     env,
		EnvVars:     env,
		AppName:     appName,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	// FromModel doesn't need a datastore, for now...
	ds := datastore.NewMockInit(nil, nil, nil)

	a := New(ds, ds, new(mqs.Mock), AgentTypeFull)
	defer a.Close()

	callI, err := a.GetCall(FromModel(cm))
	if err != nil {
		t.Fatal(err)
	}

	// make sure headers seem reasonable
	req := callI.(*call).req

	// NOTE these are added as is atm, and if the env vars were comma joined
	// they are not again here comma separated.
	expectedHeaders := make(http.Header)
	for k, v := range env {
		expectedHeaders.Add(k, v)
	}

	for k, vs := range req.Header {
		for i, v := range expectedHeaders[k] {
			if i >= len(vs) || vs[i] != v {
				t.Fatal("header mismatch", k, vs)
			}
		}
		delete(expectedHeaders, k)
	}

	if len(expectedHeaders) > 0 {
		t.Fatal("got extra headers, bad")
	}

	var b bytes.Buffer
	io.Copy(&b, req.Body)

	if b.String() != payload {
		t.Fatal("expected payload to match, but it was a lie")
	}
}

func TestLoggerIsStringerAndWorks(t *testing.T) {
	// TODO test limit writer, logrus writer, etc etc

	loggyloo := logrus.WithFields(logrus.Fields{"yodawg": true})
	logger := setupLogger(loggyloo)

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

	logger.Close() // idk maybe this would panic might as well call this

	// TODO we could check for the toilet to flush here to logrus
}

func TestSubmitError(t *testing.T) {
	appName := "myapp"
	path := "/error"
	image := "fnproject/error"
	const timeout = 10
	const idleTimeout = 20
	const memory = 256
	method := "GET"
	url := "http://127.0.0.1:8080/r/" + appName + path
	payload := "payload"
	typ := "sync"
	format := "default"
	env := map[string]string{
		"FN_FORMAT":   format,
		"FN_APP_NAME": appName,
		"FN_PATH":     path,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
		"ROUTE_VAR":   "BAR",
		"DOUBLE_VAR":  "BIZ, BAZ",
	}

	cm := &models.Call{
		BaseEnv:     env,
		EnvVars:     env,
		AppName:     appName,
		Path:        path,
		Image:       image,
		Type:        typ,
		Format:      format,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	// FromModel doesn't need a datastore, for now...
	ds := datastore.NewMockInit(nil, nil, nil)

	a := New(ds, ds, new(mqs.Mock), AgentTypeFull)
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
