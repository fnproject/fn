package runner

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/iron-io/functions/api/models"
	"golang.org/x/net/context"
)

func TestRunnerHello(t *testing.T) {
	runner, err := New()
	if err != nil {
		t.Fatalf("Test error during New() - %s", err)
	}

	ctx := context.Background()

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
		cfg := &Config{
			ID:      fmt.Sprintf("task-hello-%d-%d", i, time.Now().Unix()),
			Route:   test.route,
			Timeout: 5 * time.Second,
			Payload: test.payload,
			Stdout:  &stdout,
			Stderr:  &stderr,
		}

		result, err := runner.Run(ctx, cfg)
		if err != nil {
			t.Fatalf("Test %d: error during Run() - %s", i, err)
		}

		if test.expectedStatus != result.Status() {
			t.Fatalf("Test %d: expected result status to be `%s` but it was `%s`", i, test.expectedStatus, result.Status())
		}

		if !bytes.Contains(stdout.Bytes(), []byte(test.expectedOut)) {
			t.Fatalf("Test %d: expected output log to contain `%s` in `%s`", i, test.expectedOut, stdout.String())
		}

		if !bytes.Contains(stderr.Bytes(), []byte(test.expectedErr)) {
			t.Fatalf("Test %d: expected error log to contain `%s` in `%s`", i, test.expectedErr, stderr.String())
		}
	}
}

func TestRunnerError(t *testing.T) {
	runner, err := New()
	if err != nil {
		t.Fatalf("Test error during New() - %s", err)
	}

	ctx := context.Background()

	for i, test := range []struct {
		route          *models.Route
		payload        string
		expectedStatus string
		expectedOut    string
		expectedErr    string
	}{
		{&models.Route{Image: "iron/error"}, ``, "error", "", "RuntimeError"},
		{&models.Route{Image: "iron/error"}, `{"name": "test"}`, "error", "", "RuntimeError"},
	} {
		var stdout, stderr bytes.Buffer
		cfg := &Config{
			ID:      fmt.Sprintf("task-err-%d-%d", i, time.Now().Unix()),
			Route:   test.route,
			Timeout: 5 * time.Second,
			Payload: test.payload,
			Stdout:  &stdout,
			Stderr:  &stderr,
		}

		result, err := runner.Run(ctx, cfg)
		if err != nil {
			t.Fatalf("Test %d: error during Run() - %s", i, err)
		}

		if test.expectedStatus != result.Status() {
			t.Fatalf("Test %d: expected result status to be `%s` but it was `%s`", i, test.expectedStatus, result.Status())
		}

		if !bytes.Contains(stdout.Bytes(), []byte(test.expectedOut)) {
			t.Fatalf("Test %d: expected output log to contain `%s` in `%s`", i, test.expectedOut, stdout.String())
		}

		if !bytes.Contains(stderr.Bytes(), []byte(test.expectedErr)) {
			t.Fatalf("Test %d: expected error log to contain `%s` in `%s`", i, test.expectedErr, stderr.String())
		}
	}
}
