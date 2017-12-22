package tests

import (
	"bytes"
	"encoding/json"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"
)

type JSONResponse struct {
	Message string `json:"message"`
}

func TestFnJSONFormats(t *testing.T) {
	t.Parallel()
	s := SetupDefaultSuite()

	// TODO(treeder): put image in fnproject @ dockerhub
	image := "denismakogon/test-hot-json-go:0.0.1"
	format := "json"
	route := "/test-hot-json-go"

	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	CreateRoute(t, s.Context, s.Client, s.AppName, route, image, "sync",
		format, s.Timeout, s.IdleTimeout, s.RouteConfig, s.RouteHeaders)

	u := url.URL{
		Scheme: "http",
		Host:   Host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	b, _ := json.Marshal(&struct {
		Name string `json:"name"`
	}{
		Name: "Jimmy",
	})
	content := bytes.NewBuffer(b)
	output := &bytes.Buffer{}
	headers, err := CallFN(u.String(), content, output, "POST", []string{})
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

	DeleteApp(t, s.Context, s.Client, s.AppName)
}
