package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore/datastoretest"
	"github.com/fnproject/fn/api/models"
)

// See fn-test-utils for json response
func getEchoContent(respBytes []byte) (string, error) {

	var respJs map[string]interface{}

	err := json.Unmarshal(respBytes, &respJs)
	if err != nil {
		return "", err
	}

	req, ok := respJs["request"].(map[string]interface{})
	if !ok {
		return "", errors.New("unexpected json: request map")
	}

	echo, ok := req["echoContent"].(string)
	if !ok {
		return "", errors.New("unexpected json: echoContent string")
	}

	return echo, nil
}

// See fn-test-utils for json response
func getConfigContent(key string, respBytes []byte) (string, error) {

	var respJs map[string]interface{}

	err := json.Unmarshal(respBytes, &respJs)
	if err != nil {
		return "", err
	}

	cfg, ok := respJs["config"].(map[string]interface{})
	if !ok {
		return "", errors.New("unexpected json: config map")
	}

	val, ok := cfg[key].(string)
	if !ok {
		return "", fmt.Errorf("unexpected json: %s string", key)
	}

	return val, nil
}

type systemTestResourceProvider struct {
	datastoretest.ResourceProvider
}

func (rp *systemTestResourceProvider) ValidFn(appID string) *models.Fn {
	fn := rp.ResourceProvider.ValidFn(appID)
	fn.Memory = memory
	fn.Image = image
	return fn
}

var rp = &systemTestResourceProvider{
	ResourceProvider: datastoretest.NewBasicResourceProvider(),
}

func TestCanExecuteFunctionViaTrigger(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	app := ensureApp(t, rp.ValidApp())
	fn := ensureFn(t, rp.ValidFn(app.ID))
	trigger := ensureTrigger(t, rp.ValidTrigger(app.ID, fn.ID))

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "t", app.Name, trigger.Source)

	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callTrigger(ctx, u.String(), content, output, "POST")
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	echo, err := getEchoContent(output.Bytes())
	if err != nil || echo != "HelloWorld" {
		t.Fatalf("getEchoContent/HelloWorld check failed on %v", output)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}

	// Now let's check FN_CHEESE, since LB and runners have override/extension mechanism
	// to insert FN_CHEESE into config
	cheese, err := getConfigContent("FN_CHEESE", output.Bytes())
	if err != nil || cheese != "Tete de Moine" {
		t.Fatalf("getConfigContent/FN_CHEESE check failed (%v) on %v", err, output)
	}

	// Now let's check FN_WINE, since runners have override to insert this.
	wine, err := getConfigContent("FN_WINE", output.Bytes())
	if err != nil || wine != "1982 Margaux" {
		t.Fatalf("getConfigContent/FN_WINE check failed (%v) on %v", err, output)
	}
}

func TestCanExecuteTriggerBigOutput(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	app := ensureApp(t, rp.ValidApp())
	fn := ensureFn(t, rp.ValidFn(app.ID))
	trigger := ensureTrigger(t, rp.ValidTrigger(app.ID, fn.ID))

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "t", app.Name, trigger.Source)

	// Approx 5.3MB output
	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true, "trailerRepeat": 410000}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callTrigger(ctx, u.String(), content, output, "POST")
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	t.Logf("getEchoContent/HelloWorld size %d", len(output.Bytes()))

	echo, err := getEchoContent(output.Bytes())
	if err != nil || echo != "HelloWorld" {
		t.Fatalf("getEchoContent/HelloWorld check failed on %v", output)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestCanExecuteTriggerTooBigOutput(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	app := ensureApp(t, rp.ValidApp())
	fn := ensureFn(t, rp.ValidFn(app.ID))
	trigger := ensureTrigger(t, rp.ValidTrigger(app.ID, fn.ID))

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "t", app.Name, trigger.Source)

	// > 6MB output
	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true, "trailerRepeat": 600000}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callTrigger(ctx, u.String(), content, output, "POST")
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	exp := models.ErrFunctionResponseTooBig.Error()
	actual := output.String()

	if !strings.Contains(exp, actual) || len(exp) != len(actual) {
		t.Fatalf("Assertion error.\n\tExpected: %v\n\tActual: %v", exp, output.String())
	}

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestCanExecuteTriggerEmptyOutput(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	app := ensureApp(t, rp.ValidApp())
	fn := ensureFn(t, rp.ValidFn(app.ID))
	trigger := ensureTrigger(t, rp.ValidTrigger(app.ID, fn.ID))

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "t", app.Name, trigger.Source)

	// empty body output
	body := `{"sleepTime": 0, "isDebug": true, "isEmptyBody": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callTrigger(ctx, u.String(), content, output, "POST")
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	actual := output.String()

	if 0 != len(actual) {
		t.Fatalf("Assertion error.\n\tExpected empty\n\tActual: %v", output.String())
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestBasicTriggerConcurrentExecution(t *testing.T) {
	buf := setLogBuffer()
	defer func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	app := ensureApp(t, rp.ValidApp())
	fn := ensureFn(t, rp.ValidFn(app.ID))
	trigger := ensureTrigger(t, rp.ValidTrigger(app.ID, fn.ID))

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "t", app.Name, trigger.Source)

	results := make(chan error)
	concurrentFuncs := 10
	for i := 0; i < concurrentFuncs; i++ {
		go func() {
			body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
			content := bytes.NewBuffer([]byte(body))
			output := &bytes.Buffer{}
			resp, err := callTrigger(ctx, u.String(), content, output, "POST")
			if err != nil {
				results <- fmt.Errorf("Got unexpected error: %v", err)
				return
			}

			echo, err := getEchoContent(output.Bytes())
			if err != nil || echo != "HelloWorld" {
				results <- fmt.Errorf("Assertion error.\n\tActual: %v", output.String())
				return
			}
			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("StatusCode check failed on %v", resp.StatusCode)
				return
			}

			results <- nil
		}()
	}
	for i := 0; i < concurrentFuncs; i++ {
		err := <-results
		if err != nil {
			t.Fatalf("Error in basic concurrency execution test: %v", err)
		}
	}

}

func callTrigger(ctx context.Context, u string, content io.Reader, output io.Writer, method string) (*http.Response, error) {
	if method == "" {
		if content == nil {
			method = "GET"
		} else {
			method = "POST"
		}
	}

	req, err := http.NewRequest(method, u, content)
	if err != nil {
		return nil, fmt.Errorf("error running route: %s", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error running route: %s", err)
	}

	io.Copy(output, resp.Body)

	return resp, nil
}

func getAPIURL() (string, *url.URL) {
	u, err := url.Parse(APIAddress)
	if err != nil {
		log.Fatalf("Couldn't parse API URL: %s error: %s", APIAddress, err)
	}
	return APIAddress, u
}

func host() string {
	u, _ := getAPIURL()
	return u
}

const (
	appName   = "systemtestapp"
	routeName = "/systemtestroute"
	image     = "fnproject/fn-test-utils"
	memory    = 128
	typ       = "sync"
)

func ensureApp(t *testing.T, app *models.App) *models.App {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(app)
	if err != nil {
		t.Fatal("error encoding body", err)
	}

	urlStr := host() + "/v2/apps"
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatal("error creating url", urlStr, err)
	}

	req, err := http.NewRequest("POST", u.String(), &buf)
	if err != nil {
		t.Fatal("error creating request", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("error creating route", err)
	}

	buf.Reset()
	io.Copy(&buf, resp.Body)
	if resp.StatusCode != 200 {
		t.Fatal("error creating/updating app or otherwise ensuring it exists:", resp.StatusCode, buf.String())
	}

	var appOut models.App
	err = json.NewDecoder(&buf).Decode(&appOut)
	if err != nil {
		t.Fatal("error decoding response")
	}

	return &appOut
}

func ensureFn(t *testing.T, fn *models.Fn) *models.Fn {

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(fn)
	if err != nil {
		t.Fatal("error encoding body", err)
	}

	urlStr := host() + "/v2/fns"
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatal("error creating url", urlStr, err)
	}

	req, err := http.NewRequest("POST", u.String(), &buf)
	if err != nil {
		t.Fatal("error creating request", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("error creating route", err)
	}

	buf.Reset()
	io.Copy(&buf, resp.Body)
	if resp.StatusCode != 200 {
		t.Fatal("error creating/updating app or otherwise ensuring it exists:", resp.StatusCode, buf.String())
	}

	var fnOut models.Fn
	err = json.NewDecoder(&buf).Decode(&fnOut)
	if err != nil {
		t.Fatal("error decoding response")
	}

	return &fnOut

}

func ensureTrigger(t *testing.T, trigger *models.Trigger) *models.Trigger {

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(trigger)
	if err != nil {
		t.Fatal("error encoding body", err)
	}

	urlStr := host() + "/v2/triggers"
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatal("error creating url", urlStr, err)
	}

	req, err := http.NewRequest("POST", u.String(), &buf)
	if err != nil {
		t.Fatal("error creating request", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("error creating route", err)
	}

	buf.Reset()
	io.Copy(&buf, resp.Body)
	if resp.StatusCode != 200 {
		t.Fatal("error creating/updating app or otherwise ensuring it exists:", resp.StatusCode, buf.String())
	}

	var triggerOut models.Trigger
	err = json.NewDecoder(&buf).Decode(&triggerOut)
	if err != nil {
		t.Fatal("error decoding response")
	}

	return &triggerOut
}
