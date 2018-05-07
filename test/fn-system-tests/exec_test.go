package tests

import (
	"bytes"
	"fmt"
	"net/url"
	//"os"
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

func configureSetup(t *testing.T, fntype, format, image string) (*apiutils.TestHarness, url.URL) {
	s := apiutils.SetupHarness()
	s.Image = image
	s.GivenAppExists(t, &sdkmodels.App{Name: s.AppName})

	rt := s.BasicRoute()
	rt.Type = fntype
	rt.Format = format

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

	return s, u

}

func TestCanExecuteFunction(t *testing.T) {
	s, u := configureSetup(t, "sync", "default", "fnproject/hello")
	defer s.Cleanup()

	content := &bytes.Buffer{}
	output := &bytes.Buffer{}
	_, err := apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	expectedOutput := "Hello World!\n"
	actual := output.String()
	if !strings.Contains(expectedOutput, actual) || len(expectedOutput) != len(actual) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}

func TestBasicConcurrentExecution(t *testing.T) {
	SystemTweaker().ChangeNodeCapacities(512)
	defer SystemTweaker().RestoreInitialNodeCapacities()

	s, u := configureSetup(t, "sync", "default", "fnproject/hello")
	defer s.Cleanup()

	results := make(chan error)
	concurrentFuncs := 10
	for i := 0; i < concurrentFuncs; i++ {
		go func() {
			content := &bytes.Buffer{}
			output := &bytes.Buffer{}
			_, err := apiutils.CallFN(u.String(), content, output, "POST", []string{})
			if err != nil {
				results <- fmt.Errorf("Got unexpected error: %v", err)
				return
			}
			expectedOutput := "Hello World!\n"
			actual := output.String()
			if !strings.Contains(expectedOutput, actual) || len(expectedOutput) != len(actual) {
				results <- fmt.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
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
	// Set the capacity to 0 so we always look out of capacity.
	SystemTweaker().ChangeNodeCapacities(0)
	defer SystemTweaker().RestoreInitialNodeCapacities()

	s, u := configureSetup(t, "sync", "default", "fnproject/hello")
	defer s.Cleanup()

	content := &bytes.Buffer{}
	output := &bytes.Buffer{}
	_, err := apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		if err != apimodels.ErrCallTimeoutServerBusy {
			t.Errorf("Got unexpected error: %v", err)
		}
	}
	expectedOutput := "{\"error\":{\"message\":\"Timed out - server too busy\"}}\n"
	actual := output.String()
	if !strings.Contains(expectedOutput, actual) || len(expectedOutput) != len(actual) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}

func TestHotJSONWithInput(t *testing.T) {
	s, u := configureSetup(t, "sync", "json", "fnproject/system-tests-hello-go-json")
	defer s.Cleanup()

	content := &bytes.Buffer{}
	output := &bytes.Buffer{}

	in := "{\"Name\": \"Test\"}"
	content.WriteString(in)
	_, err := apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	expectedOutput := "Hello Test!"
	actual := output.String()
	if !strings.Contains(expectedOutput, actual) || len(expectedOutput) != len(actual) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}

func TestHotJSONNoInput(t *testing.T) {
	s, u := configureSetup(t, "sync", "json", "fnproject/system-tests-hello-go-json")
	defer s.Cleanup()

	content := &bytes.Buffer{}
	output := &bytes.Buffer{}
	in := "{}"
	content.WriteString(in)
	_, err := apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	expectedOutput := "Hello World!"
	actual := output.String()
	if !strings.Contains(expectedOutput, actual) || len(expectedOutput) != len(actual) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}

func TestHotHTTPWithInput(t *testing.T) {
	s, u := configureSetup(t, "sync", "http", "fnproject/system-tests-hello-go-http")
	defer s.Cleanup()

	content := &bytes.Buffer{}
	output := &bytes.Buffer{}

	in := "{\"Name\": \"Test\"}"
	content.WriteString(in)
	_, err := apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	expectedOutput := "Hello Test!"
	actual := output.String()
	if !strings.Contains(expectedOutput, actual) || len(expectedOutput) != len(actual) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}

func TestHotHTTPNoInput(t *testing.T) {
	s, u := configureSetup(t, "sync", "http", "fnproject/system-tests-hello-go-http")
	defer s.Cleanup()

	content := &bytes.Buffer{}
	output := &bytes.Buffer{}

	_, err := apiutils.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	expectedOutput := "Hello World!"
	actual := output.String()
	if !strings.Contains(expectedOutput, actual) || len(expectedOutput) != len(actual) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}
}
