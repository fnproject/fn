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
	"net/http"
	"os"
)

type JSONResponse struct {
	Message string `json:"message"`
}

func runAndAssert(t *testing.T, s *fnTest.SuiteSetup, fnRoute, fnImage,
	fnFormat string, requestPayload interface{}, responsePayload interface{}) (*bytes.Buffer, *http.Response) {

	fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, fnRoute, fnImage, "sync",
		fnFormat, s.RouteConfig, s.RouteHeaders)

	u := url.URL{
		Scheme: "http",
		Host:   fnTest.Host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, fnRoute)

	b, _ := json.Marshal(requestPayload)
	content := bytes.NewBuffer(b)
	output := &bytes.Buffer{}

	response, err := fnTest.CallFN(u.String(), content, output, "POST", []string{})

	if err != nil {
		t.Errorf("Got unexpected error: %v", err)
	}
	json.Unmarshal(output.Bytes(), responsePayload)

	fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)

	return output, response

}

func TestFDKFormatSmallBody(t *testing.T) {

	FDKImage := os.Getenv("FDK_FUNCTION_IMAGE")
	if FDKImage == "" {
		t.Error("Please set FDK-based function image to test")
	}
	formats := []string{"http", "json"}

	helloJohnPayload := &struct {
		Name string `json:"name"`
	}{
		Name: "Jimmy",
	}
	helloJohnExpectedOutput := "Hello Jimmy"
	for _, format := range formats {

		// echo function:
		// payload:
		//    {
		//        "name": "John"
		//    }
		// response:
		//    "Hello John"
		t.Run(fmt.Sprintf("test-fdk-%v-small-body", format), func(t *testing.T) {

			t.Parallel()
			s := fnTest.SetupDefaultSuite()
			route := fmt.Sprintf("/test-fdk-%v-format-small-body", format)

			responsePayload := &JSONResponse{}
			output, response := runAndAssert(t, s, route, FDKImage, format, helloJohnPayload, responsePayload)

			if !strings.Contains(helloJohnExpectedOutput, responsePayload.Message) {
				t.Errorf("Output assertion error.\n\tExpected: %v\n\tActual: %v", helloJohnExpectedOutput, output.String())
			}
			if response.StatusCode != 200 {
				t.Errorf("Status code assertion error.\n\tExpected: %v\n\tActual: %v", 200, response.StatusCode)
			}

			expectedHeaderNames := []string{"Content-Type", "Content-Length"}
			expectedHeaderValues := []string{"text/plain; charset=utf-8", strconv.Itoa(output.Len())}
			for i, name := range expectedHeaderNames {
				actual := response.Header.Get(name)
				expected := expectedHeaderValues[i]
				if !strings.Contains(expected, actual) {
					t.Errorf("HTTP header assertion error for %v."+
						"\n\tExpected: %v\n\tActual: %v", name, expected, actual)
				}
			}

		})
	}
}
