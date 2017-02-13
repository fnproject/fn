package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner/task"
)

func TestRunnerHello(t *testing.T) {
	buf := setLogBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner, err := New(ctx, NewFuncLogger(), NewMetricLogger())
	if err != nil {
		t.Fatalf("Test error during New() - %s", err)
	}

	for i, test := range []struct {
		route          *models.Route
		payload        string
		expectedStatus string
		expectedOut    string
		expectedErr    string
	}{
		{&models.Route{Image: "iron/hello"}, ``, "success", "Hello World!", ""},
		{&models.Route{Image: "iron/hello"}, `{"name": "test"}`, "success", "Hello test!", ""},
	} {
		var stdout, stderr bytes.Buffer
		cfg := &task.Config{
			ID:      fmt.Sprintf("hello-%d-%d", i, time.Now().Unix()),
			Image:   test.route.Image,
			Timeout: 10 * time.Second,
			Stdin:   strings.NewReader(test.payload),
			Stdout:  &stdout,
			Stderr:  &stderr,
		}

		result, err := runner.Run(ctx, cfg)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test %d: error during Run() - %s", i, err)
		}

		if test.expectedStatus != result.Status() {
			t.Log(buf.String())
			t.Fatalf("Test %d: expected result status to be `%s` but it was `%s`", i, test.expectedStatus, result.Status())
		}

		if !bytes.Contains(stdout.Bytes(), []byte(test.expectedOut)) {
			t.Log(buf.String())
			t.Fatalf("Test %d: expected output log to contain `%s` in `%s`", i, test.expectedOut, stdout.String())
		}

		if !bytes.Contains(stderr.Bytes(), []byte(test.expectedErr)) {
			t.Log(buf.String())
			t.Fatalf("Test %d: expected error log to contain `%s` in `%s`", i, test.expectedErr, stderr.String())
		}
	}
}

func TestRunnerError(t *testing.T) {
	buf := setLogBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner, err := New(ctx, NewFuncLogger(), NewMetricLogger())
	if err != nil {
		t.Fatalf("Test error during New() - %s", err)
	}

	for i, test := range []struct {
		route          *models.Route
		payload        string
		expectedStatus string
		expectedOut    string
		expectedErr    string
	}{
		{&models.Route{Image: "iron/error"}, ``, "error", "", ""},
		{&models.Route{Image: "iron/error"}, `{"name": "test"}`, "error", "", ""},
	} {
		var stdout, stderr bytes.Buffer
		cfg := &task.Config{
			ID:      fmt.Sprintf("err-%d-%d", i, time.Now().Unix()),
			Image:   test.route.Image,
			Timeout: 10 * time.Second,
			Stdin:   strings.NewReader(test.payload),
			Stdout:  &stdout,
			Stderr:  &stderr,
		}

		result, err := runner.Run(ctx, cfg)
		if err != nil {
			t.Log(buf.String())
			t.Fatalf("Test %d: error during Run() - %s", i, err)
		}

		if test.expectedStatus != result.Status() {
			t.Log(buf.String())
			t.Fatalf("Test %d: expected result status to be `%s` but it was `%s`", i, test.expectedStatus, result.Status())
		}

		if !bytes.Contains(stdout.Bytes(), []byte(test.expectedOut)) {
			t.Log(buf.String())
			t.Fatalf("Test %d: expected output log to contain `%s` in `%s`", i, test.expectedOut, stdout.String())
		}

		if !bytes.Contains(stderr.Bytes(), []byte(test.expectedErr)) {
			t.Log(buf.String())
			t.Fatalf("Test %d: expected error log to contain `%s` in `%s`", i, test.expectedErr, stderr.String())
		}
	}
}
