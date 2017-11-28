package tests

import (
	"bytes"
	"encoding/json"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"

	fnTest "github.com/fnproject/fn/test"
)

type JSONResponse struct {
	Message string `json:"message"`
}

func TestFnFormats(t *testing.T) {

	t.Run("test-json-format", func(t *testing.T) {
		t.Parallel()
		s := fnTest.SetupDefaultSuite()

		// TODO(treeder): put image in fnproject @ dockerhub
		image := "denismakogon/test-hot-json-go:0.0.1"
		format := "json"
		route := "/test-hot-json-go"

		fnTest.CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
		fnTest.CreateRoute(t, s.Context, s.Client, s.AppName, route, image, "sync",
			format, s.RouteConfig, s.RouteHeaders)

		u := url.URL{
			Scheme: "http",
			Host:   fnTest.Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

		b, _ := json.Marshal(&struct {
			Name string `json:"name"`
		}{
			Name: "Jimmy",
		})
		content := bytes.NewBuffer(b)
		output := &bytes.Buffer{}
		headers, err := fnTest.CallFN(u.String(), content, output, "POST", []string{})
		if err != nil {
			t.Errorf("Got unexpected error: %v", err)
		}

		msg := &JSONResponse{}
		json.Unmarshal(output.Bytes(), msg)
		expectedOutput := "Hello Jimmy"
		if !strings.Contains(expectedOutput, msg.Message) {
			t.Errorf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
		}

		expectedHeaderNames := []string{"Content-Type", "Content-Length"}
		expectedHeaderValues := []string{"application/json; charset=utf-8", strconv.Itoa(output.Len())}
		for i, name := range expectedHeaderNames {
			actual := headers.Get(name)
			expected := expectedHeaderValues[i]
			if !strings.Contains(expected, actual) {
				t.Errorf("HTTP header assertion error for %v."+
					"\n\tExpected: %v\n\tActual: %v", name, expected, actual)
			}
		}

		fnTest.DeleteApp(t, s.Context, s.Client, s.AppName)

	})

}
