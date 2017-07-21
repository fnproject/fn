package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/funcy/functions_go/client/call"
	"github.com/funcy/functions_go/client/operations"
)

type ErrMsg struct {
	Message string `json:"message"`
}

type TimeoutBody struct {
	Error  ErrMsg `json:"error"`
	CallID string `json:"request_id"`
}

func CallAsync(t *testing.T, u url.URL, content io.Reader) string {
	output := &bytes.Buffer{}
	err := CallFN(u.String(), content, output, "POST", []string{})
	if err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}

	expectedOutput := "call_id"
	if !strings.Contains(output.String(), expectedOutput) {
		t.Fatalf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
	}

	type CallID struct {
		CallID string `json:"call_id"`
	}

	callID := &CallID{}
	json.NewDecoder(output).Decode(callID)

	if callID.CallID == "" {
		t.Fatalf("`call_id` not suppose to be empty string")
	}
	t.Logf("Async execution call ID: %v", callID.CallID)
	return callID.CallID
}

func TestRouteExecutions(t *testing.T) {
	s := SetupDefaultSuite()
	newRouteType := "async"

	CreateApp(t, s.Context, s.Client, s.AppName, map[string]string{})
	CreateRoute(t, s.Context, s.Client, s.AppName, s.RoutePath, s.Image, "sync",
		s.RouteConfig, s.RouteHeaders)

	u := url.URL{
		Scheme: "http",
		Host:   Host(),
	}
	u.Path = path.Join(u.Path, "r", s.AppName, s.RoutePath)

	t.Run("run-sync-funcy/hello-no-input", func(t *testing.T) {
		content := &bytes.Buffer{}
		output := &bytes.Buffer{}
		err := CallFN(u.String(), content, output, "POST", []string{})
		if err != nil {
			t.Fatalf("Got unexpected error: %v", err)
		}
		expectedOutput := "Hello World!\n"
		if !strings.Contains(expectedOutput, output.String()) {
			t.Fatalf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
		}
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("run-sync-funcy/hello-with-input", func(t *testing.T) {
		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Name string
		}{Name: "John"})
		output := &bytes.Buffer{}
		err := CallFN(u.String(), content, output, "POST", []string{})
		if err != nil {
			t.Fatalf("Got unexpected error: %v", err)
		}
		expectedOutput := "Hello John!\n"
		if !strings.Contains(expectedOutput, output.String()) {
			t.Fatalf("Assertion error.\n\tExpected: %v\n\tActual: %v", expectedOutput, output.String())
		}
		t.Logf("Test `%v` passed.", t.Name())
	})

	_, err := UpdateRoute(
		t, s.Context, s.Client,
		s.AppName, s.RoutePath,
		s.Image, newRouteType, s.Format,
		s.Memory, s.RouteConfig, s.RouteHeaders, "")

	CheckRouteResponseError(t, err)

	t.Run("run-async-funcy/hello", func(t *testing.T) {
		CallAsync(t, u, &bytes.Buffer{})
		t.Logf("Test `%v` passed.", t.Name())
	})

	t.Run("run-async-funcy/hello-with-status-check", func(t *testing.T) {
		callID := CallAsync(t, u, &bytes.Buffer{})
		time.Sleep(time.Second * 2)
		cfg := &call.GetCallsCallParams{
			Call:    callID,
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		callResponse, err := s.Client.Call.GetCallsCall(cfg)
		if err != nil {
			switch err.(type) {
			case *call.GetCallsCallNotFound:
				msg := err.(*call.GetCallsCallNotFound).Payload.Error.Message
				t.Fatalf("Unexpected error occurred: %v.", msg)
			}
		}
		callObject := callResponse.Payload.Call

		if callObject.AppName != s.AppName {
			t.Fatalf("Call object app name mismatch.\n\tExpected: %v\n\tActual:%v", s.AppName, callObject.AppName)
		}
		if callObject.ID != callID {
			t.Fatalf("Call object ID mismatch.\n\tExpected: %v\n\tActual:%v", callID, callObject.ID)
		}
		if callObject.Path != s.RoutePath {
			t.Fatalf("Call object route path mismatch.\n\tExpected: %v\n\tActual:%v", s.RoutePath, callObject.Path)
		}
		if callObject.Status != "success" {
			t.Fatalf("Call object status mismatch.\n\tExpected: %v\n\tActual:%v", "success", callObject.Status)
		}

	})

	DeleteRoute(t, s.Context, s.Client, s.AppName, s.RoutePath)

	routePath := "/timeout"
	image := "funcy/timeout:0.0.1"
	routeType := "sync"
	CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
		s.RouteConfig, s.RouteHeaders)

	t.Run("exec-timeout-test", func(t *testing.T) {

		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)

		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Seconds int64 `json:"seconds"`
		}{Seconds: 31})
		output := &bytes.Buffer{}

		CallFN(u.String(), content, output, "POST", []string{})

		if !strings.Contains(output.String(), "Timed out") {
			t.Fatalf("Must fail because of timeout, but got error message: %v", output.String())
		}
		tB := &TimeoutBody{}

		json.NewDecoder(output).Decode(tB)

		cfg := &call.GetCallsCallParams{
			Call:    tB.CallID,
			Context: s.Context,
		}
		cfg.WithTimeout(time.Second * 60)
		callObj, err := s.Client.Call.GetCallsCall(cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if !strings.Contains("timeout", callObj.Payload.Call.Status) {
			t.Fatalf("Call status mismatch.\n\tExpected: %v\n\tActual: %v",
				"output", "callObj.Payload.Call.Status")
		}

		t.Logf("Test `%v` passed.", t.Name())
	})
	DeleteRoute(t, s.Context, s.Client, s.AppName, routePath)

	routePath = "/multi-log"
	image = "funcy/multi-log:0.0.1"
	routeType = "async"
	CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
		s.RouteConfig, s.RouteHeaders)

	t.Run("exec-multi-log-test", func(t *testing.T) {
		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)

		callID := CallAsync(t, u, &bytes.Buffer{})
		time.Sleep(7 * time.Second)

		cfg := &operations.GetCallsCallLogParams{
			Call:    callID,
			Context: s.Context,
		}

		logObj, err := s.Client.Operations.GetCallsCallLog(cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if logObj.Payload.Log.Log == "" {
			t.Fatalf("Log entry must not be empty!")
		}
		if !strings.Contains(logObj.Payload.Log.Log, "First line") {
			t.Fatalf("Log entry must contain `First line` "+
				"string, but got: %v", logObj.Payload.Log.Log)
		}
		if !strings.Contains(logObj.Payload.Log.Log, "Second line") {
			t.Fatalf("Log entry must contain `Second line` "+
				"string, but got: %v", logObj.Payload.Log.Log)
		}
	})

	DeleteRoute(t, s.Context, s.Client, s.AppName, routePath)

	routePath = "/log"
	image = "funcy/log:0.0.1"
	routeType = "async"
	CreateRoute(t, s.Context, s.Client, s.AppName, routePath, image, routeType,
		s.RouteConfig, s.RouteHeaders)

	t.Run("exec-log-test", func(t *testing.T) {
		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)
		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Size int
		}{Size: 20})

		callID := CallAsync(t, u, content)
		time.Sleep(5 * time.Second)

		cfg := &operations.GetCallsCallLogParams{
			Call:    callID,
			Context: s.Context,
		}

		_, err := s.Client.Operations.GetCallsCallLog(cfg)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

	})

	t.Run("exec-oversized-log-test", func(t *testing.T) {
		t.Skip("Skipped until fix for https://gitlab-odx.oracle.com/odx/functions/issues/86.")
		size := 1 * 1024 * 1024 * 1024
		u := url.URL{
			Scheme: "http",
			Host:   Host(),
		}
		u.Path = path.Join(u.Path, "r", s.AppName, routePath)
		content := &bytes.Buffer{}
		json.NewEncoder(content).Encode(struct {
			Size int
		}{Size: size}) //exceeding log by 1 symbol

		callID := CallAsync(t, u, content)
		time.Sleep(5 * time.Second)

		cfg := &operations.GetCallsCallLogParams{
			Call:    callID,
			Context: s.Context,
		}

		logObj, err := s.Client.Operations.GetCallsCallLog(cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if len(logObj.Payload.Log.Log) >= size {
			t.Fatalf("Log entry suppose to be truncated up to expected size %v, got %v",
				size/1024, len(logObj.Payload.Log.Log))
		}
	})

	DeleteRoute(t, s.Context, s.Client, s.AppName, routePath)

	DeleteApp(t, s.Context, s.Client, s.AppName)
}
