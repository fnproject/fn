package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
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
		{&models.Route{Image: "fnproject/hello"}, ``, "success", "Hello World!", "", id.New().String()},
		{&models.Route{Image: "fnproject/hello"}, `{"name": "test"}`, "success", "Hello test!", "", id.New().String()},
	} {
		var stdout, stderr bytes.Buffer
		cfg := &models.Task{
			ID:      test.taskID,
			Image:   test.route.Image,
			Timeout: 10,
			Memory:  128,
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
		{&models.Route{Image: "fnproject/error"}, ``, "error", "", "", id.New().String()},
		{&models.Route{Image: "fnproject/error"}, `{"name": "test"}`, "error", "", "", id.New().String()},
	} {
		var stdout, stderr bytes.Buffer
		cfg := &models.Task{
			ID:      fmt.Sprintf("err-%d-%d", i, time.Now().Unix()),
			Image:   test.route.Image,
			Timeout: 10,
			Memory:  128,
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

func TestRunnerMemory(t *testing.T) {
	// make sure we get MB out of a task.Config when turned into a containerTask
	// (so if Config.Memory changes to not be MB we hear about it)

	cfg := &models.Task{
		Memory: 128,
	}

	task := &containerTask{cfg: cfg}

	const exp = 128 * 1024 * 1024
	if task.Memory() != exp {
		t.Fatalf("Expected Memory to return %v but got %v", exp, task.Memory())
	}
}
