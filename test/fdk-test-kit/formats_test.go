package fdk_test_kit

import (
	"bytes"
	"encoding/json"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"

	"fmt"
	fnTest "github.com/fnproject/fn/test"
	"os"
)

type JSONResponse struct {
	Message string `json:"message"`
}

func runAndAssert(t *testing.T, s *fnTest.SuiteSetup, fnRoute, fnImage, fnFormat string) {
	fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, fnRoute, fnImage, "sync",
		fnFormat, s.RouteConfig, s.RouteHeaders)

	u := url.URL{
		Scheme: "http",
		Host:   fnTest.Host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, fnRoute)

	b, _ := json.Marshal(&struct {
		Name string `json:"name"`
	}{
		Name: "Jimmy",
	})
	t.Log(u.String())
	content := bytes.NewBuffer(b)
	output := &bytes.Buffer{}
	headers, err := fnTest.CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	t.Log(output.String())
	msg := &JSONResponse{}
	json.Unmarshal(output.Bytes(), msg)
	expectedOutput := "Hello Jimmy"
	if !strings.Contains(expectedOutput, msg.Message) {
		t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}

	expectedHeaderNames := []string{"Content-Type", "Content-Length"}
	expectedHeaderValues := []string{"text/plain; charset=utf-8", strconv.Itoa(output.Len())}
	for i, name := range expectedHeaderNames {
		actual := headers.Get(name)
		expected := expectedHeaderValues[i]
		if !strings.Contains(expected, actual) {
			t.Errorf("HTTP header assertion error for %v."+
				"\n\tExpected: %v\n\tActual: %v", name, expected, actual)
		}
	}

	fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)
}

func TestFnJSONFormat(t *testing.T) {

	RegistryNameSpace := os.Getenv("FN_REGISTRY")
	if RegistryNameSpace == "" {
		RegistryNameSpace = "fnproject"
	}

	FDKs := os.Getenv("TEST_FDKS")
	if FDKs == "" {
		// since we support Python, Go, Ruby FDKs maybe we should default to them
		t.Skip("Nothing to test. TEST_FDKS env var is empty")
	}
	fdkList := strings.Split(FDKs, ",")

	// how many times should we run single test to ensure that FDK can handle requests normally?
	for _, fdkLang := range fdkList {

		// echo function:
		// payload:
		// {
		//     "name": "John"
		// }
		t.Run(fmt.Sprintf("test-fdk-%v-json-format", fdkLang), func(t *testing.T) {
			t.Parallel()
			s := fnTest.SetupDefaultSuite()
			image := fmt.Sprintf("%v/test-fdk-%v-json-format:0.0.1", RegistryNameSpace, fdkLang)
			format := "json"
			route := fmt.Sprintf("/test-fdk-%v-json-format", fdkLang)

			runAndAssert(t, s, route, image, format)
		})

	}
}

func TestFnHTTPFormat(t *testing.T) {

	RegistryNameSpace := os.Getenv("FN_REGISTRY")
	if RegistryNameSpace == "" {
		RegistryNameSpace = "fnproject"
	}

	FDKs := os.Getenv("TEST_FDKS")
	if FDKs == "" {
		// since we support Python, Go, Ruby FDKs maybe we should default to them
		t.Skip("Nothing to test. TEST_FDKS env var is empty")
	}
	fdkList := strings.Split(FDKs, ",")

	s := fnTest.SetupDefaultSuite()
	fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})

	// how many times should we run single test to ensure that FDK can handle requests normally?
	for _, fdkLang := range fdkList {

		// echo function:
		// payload:
		// {
		//     "name": "John"
		// }
		t.Run(fmt.Sprintf("test-fdk-%v-http-format", fdkLang), func(t *testing.T) {
			t.Parallel()
			s := fnTest.SetupDefaultSuite()

			image := fmt.Sprintf("%v/test-fdk-%v-http-format:0.0.1", RegistryNameSpace, fdkLang)
			format := "http"
			route := fmt.Sprintf("/test-fdk-%v-http-format", fdkLang)

			runAndAssert(t, s, route, image, format)
		})
	}

	fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)

}
