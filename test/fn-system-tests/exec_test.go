package tests

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"testing"

	apimodels "github.com/fnproject/fn/api/models"
	apiutils "github.com/fnproject/fn/test/fn-api-tests"
	sdkmodels "github.com/fnproject/fn_go/models"
)

func LB() (string, error) {
	lbURL := "http://127.0.0.1:8081"

	u, err := url.Parse(lbURL)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

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

func TestCanExecuteFunction(t *testing.T) {
	s := apiutils.SetupHarness()
	s.GivenAppExists(t, &sdkmodels.App{Name: s.AppName})
	defer s.Cleanup()

	rt := s.BasicRoute()
	rt.Image = "fnproject/fn-test-utils"
	rt.Format = "json"
	rt.Type = "sync"

	s.GivenRouteExists(t, s.AppName, rt)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	_, err = apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}

	echo, err := getEchoContent(output.Bytes())
	if err != nil || echo != "HelloWorld" {
		t.Fatalf("getEchoContent/HelloWorld check failed on %v", output)
	}
}

func TestCanExecuteBigOutput(t *testing.T) {
	s := apiutils.SetupHarness()
	s.GivenAppExists(t, &sdkmodels.App{Name: s.AppName})
	defer s.Cleanup()

	rt := s.BasicRoute()
	rt.Image = "fnproject/fn-test-utils"
	rt.Format = "json"
	rt.Type = "sync"

	s.GivenRouteExists(t, s.AppName, rt)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	// Approx 6.4MB output
	body := `{"echoContent": "HelloBigWorld", "sleepTime": 0, "isDebug": true, "trailerRepeat": 400000}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	_, err = apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}

	t.Logf("getEchoContent/HelloWorld size %d", len(output.Bytes()))

	echo, err := getEchoContent(output.Bytes())
	if err != nil || echo != "HelloBigWorld" {
		t.Fatalf("getEchoContent/HelloWorld check failed on %v", output)
	}
}

func TestBasicConcurrentExecution(t *testing.T) {

	s := apiutils.SetupHarness()

	s.GivenAppExists(t, &sdkmodels.App{Name: s.AppName})
	defer s.Cleanup()

	rt := s.BasicRoute()
	rt.Image = "fnproject/fn-test-utils"
	rt.Format = "json"
	rt.Memory = 32
	rt.Type = "sync"

	s.GivenRouteExists(t, s.AppName, rt)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	results := make(chan error)
	concurrentFuncs := 10
	for i := 0; i < concurrentFuncs; i++ {
		go func() {
			body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
			content := bytes.NewBuffer([]byte(body))
			output := &bytes.Buffer{}
			_, err = apiutils.CallFN(u.String(), content, output, "POST", []string{})
			if err != nil {
				results <- fmt.Errorf("Got unexpected error: %v", err)
				return
			}

			echo, err := getEchoContent(output.Bytes())
			if err != nil || echo != "HelloWorld" {
				results <- fmt.Errorf("Assertion error.\n\tActual: %v", output.String())
				return
			}

			results <- nil
		}()
	}
	for i := 0; i < concurrentFuncs; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Error in basic concurrency execution test: %v", err)
		}
	}

}

func TestSaturatedSystem(t *testing.T) {

	s := apiutils.SetupHarness()

	s.GivenAppExists(t, &sdkmodels.App{Name: s.AppName})
	defer s.Cleanup()

	timeout := int32(5)

	rt := s.BasicRoute()
	rt.Image = "fnproject/fn-test-utils"
	rt.Format = "json"
	rt.Timeout = &timeout
	rt.Memory = 300
	rt.Type = "sync"

	s.GivenRouteExists(t, s.AppName, rt)

	lb, err := LB()
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	u := url.URL{
		Scheme: "http",
		Host:   lb,
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	body := `{"echoContent": "HelloWorld", "sleepTime": 0, "isDebug": true}`
	content := bytes.NewBuffer([]byte(body))
	output := &bytes.Buffer{}

	_, err = apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		if err != apimodels.ErrCallTimeoutServerBusy {
			t.Errorf("Got unexpected error: %v", err)
		}
	}

	// LB may respond either with:
	//  timeout: a timeout during a call to a runner
	//  too busy: a timeout during LB retry loop
	exp1 := "{\"error\":{\"message\":\"Timed out - server too busy\"}}\n"
	exp2 := "{\"error\":{\"message\":\"Timed out\"}}\n"

	actual := output.String()

	if strings.Contains(exp1, actual) && len(exp1) == len(actual) {
	} else if strings.Contains(exp2, actual) && len(exp2) == len(actual) {
	} else {
		t.Errorf("Assertion error.\n\tExpected: %v or %v\n\tActual: %v", exp1, exp2, output.String())
	}
}
