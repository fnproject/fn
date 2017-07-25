package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gitlab-odx.oracle.com/odx/functions/api/datastore"
	"gitlab-odx.oracle.com/odx/functions/api/id"
	"gitlab-odx.oracle.com/odx/functions/api/logs"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/runner/task"
)

func TestRunnerHello(t *testing.T) {
	buf := setLogBuffer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ds := datastore.NewMock()
	fnl := logs.NewMock()
	fLogger := NewFuncLogger(fnl)
	runner, err := New(ctx, fLogger, ds)
	if err != nil {
		t.Fatalf("Test error during New() - %s", err)
	}

	for i, test := range []struct {
		route          *models.Route
		payload        string
		expectedStatus string
		expectedOut    string
		expectedErr    string
		taskID         string
	}{
		{&models.Route{Image: "funcy/hello"}, ``, "success", "Hello World!", "", id.New().String()},
		{&models.Route{Image: "funcy/hello"}, `{"name": "test"}`, "success", "Hello test!", "", id.New().String()},
	} {
		var stdout, stderr bytes.Buffer
		cfg := &task.Config{
			ID:      test.taskID,
			Image:   test.route.Image,
			Timeout: 10 * time.Second,
			Ready:   make(chan struct{}),
			Stdin:   strings.NewReader(test.payload),
			AppName: test.route.AppName,
			Stdout:  &stdout,
			Stderr:  nopCloser{&stderr},
		}

		result, err := runner.run(ctx, cfg)
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

	ds := datastore.NewMock()
	fnl := logs.NewMock()
	fLogger := NewFuncLogger(fnl)
	runner, err := New(ctx, fLogger, ds)
	if err != nil {
		t.Fatalf("Test error during New() - %s", err)
	}

	for i, test := range []struct {
		route          *models.Route
		payload        string
		expectedStatus string
		expectedOut    string
		expectedErr    string
		taskID         string
	}{
		{&models.Route{Image: "funcy/error"}, ``, "error", "", "", id.New().String()},
		{&models.Route{Image: "funcy/error"}, `{"name": "test"}`, "error", "", "", id.New().String()},
	} {
		var stdout, stderr bytes.Buffer
		cfg := &task.Config{
			ID:      fmt.Sprintf("err-%d-%d", i, time.Now().Unix()),
			Image:   test.route.Image,
			Timeout: 10 * time.Second,
			Ready:   make(chan struct{}),
			Stdin:   strings.NewReader(test.payload),
			Stdout:  &stdout,
			Stderr:  nopCloser{&stderr},
		}

		result, err := runner.run(ctx, cfg)
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
