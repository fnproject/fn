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

func TestCanExecuteFunction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rt := ensureRoute(t)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", appName, rt.Path)

	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, "POST")
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

func TestCanExecuteBigOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rt := ensureRoute(t)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", appName, rt.Path)

	// Approx 5.3MB output
	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true, "trailerRepeat": 410000}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, "POST")
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

func TestCanExecuteTooBigOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rt := ensureRoute(t)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", appName, rt.Path)

	// > 6MB output
	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true, "trailerRepeat": 600000}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, "POST")
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	exp := "{\"error\":{\"message\":\"function response too large\"}}\n"
	actual := output.String()

	if !strings.Contains(exp, actual) || len(exp) != len(actual) {
		t.Fatalf("Assertion error.\n\tExpected: %v\n\tActual: %v", exp, output.String())
	}

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("StatusCode check failed on %v", resp.StatusCode)
	}
}

func TestCanExecuteEmptyOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rt := ensureRoute(t)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", appName, rt.Path)

	// empty body output
	body := `{"sleepTime": 0, "isDebug": true, "isEmptyBody": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, "POST")
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

func TestBasicConcurrentExecution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rt := ensureRoute(t)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", appName, rt.Path)

	results := make(chan error)
	concurrentFuncs := 10
	for i := 0; i < concurrentFuncs; i++ {
		go func() {
			body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
			content := bytes.NewBuffer([]byte(body))
			output := &bytes.Buffer{}
			resp, err := callFN(ctx, u.String(), content, output, "POST")
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

func TestSaturatedSystem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rt := &models.Route{
		Path:    routeName,
		Timeout: 1,
		Image:   "fnproject/fn-test-utils",
		Format:  "json",
		Memory:  300,
		Type:    "sync",
	}
	rt = ensureRoute(t, rt)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", appName, rt.Path)

	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	resp, err := callFN(ctx, u.String(), content, output, "POST")
	if resp != nil || err == nil || ctx.Err() == nil {
		t.Fatalf("Expected response: %v err:%v", resp, err)
	}
}

func callFN(ctx context.Context, u string, content io.Reader, output io.Writer, method string) (*http.Response, error) {
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
	apiURL := getEnv("FN_API_URL", "http://localhost:8080")
	u, err := url.Parse(apiURL)
	if err != nil {
		log.Fatalf("Couldn't parse API URL: %s error: %s", apiURL, err)
	}
	return apiURL, u
}

func host() string {
	u, _ := getAPIURL()
	return u
}

const (
	appName   = "systemtestapp"
	routeName = "/systemtestroute"
	image     = "fnproject/fn-test-utils"
	format    = "json"
	memory    = 64
	typ       = "sync"
)

func ensureRoute(t *testing.T, rts ...*models.Route) *models.Route {
	var rt *models.Route
	if len(rts) > 0 {
		rt = rts[0]
	} else {
		rt = &models.Route{
			Path:   routeName + "yabbadabbadoo",
			Image:  image,
			Format: format,
			Memory: memory,
			Type:   typ,
		}
	}
	var wrapped struct {
		Route *models.Route `json:"route"`
	}

	wrapped.Route = rt

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(wrapped)
	if err != nil {
		t.Fatal("error encoding body", err)
	}

	urlStr := host() + "/v1/apps/" + appName + "/routes" + rt.Path
	u, err := url.Parse(urlStr)
	if err != nil {
		t.Fatal("error creating url", urlStr, err)
	}

	req, err := http.NewRequest("PUT", u.String(), &buf)
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

	wrapped.Route = nil
	err = json.NewDecoder(&buf).Decode(&wrapped)
	if err != nil {
		t.Fatal("error decoding response")
	}

	return wrapped.Route
}
