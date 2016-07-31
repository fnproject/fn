package runner

import (
	"bytes"
	"testing"
	"time"

	"github.com/iron-io/functions/api/models"
	"golang.org/x/net/context"
)

func TestRunnerHello(t *testing.T) {
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
		runner := New(&Config{
			Ctx:     context.Background(),
			Route:   test.route,
			Timeout: 5 * time.Second,
			Payload: test.payload,
		})

		if err := runner.Run(); err != nil {
			t.Fatalf("Test %d: error during Run() - %s", i, err)
		}

		if test.expectedStatus != runner.Status() {
			t.Fatalf("Test %d: expected result status to be `%s` but it was `%s`", i, test.expectedStatus, runner.Status())
		}

		if !bytes.Contains(runner.ReadOut(), []byte(test.expectedOut)) {
			t.Fatalf("Test %d: expected output log to contain `%s` in `%s`", i, test.expectedOut, runner.ReadOut())
		}

		if !bytes.Contains(runner.ReadErr(), []byte(test.expectedErr)) {
			t.Fatalf("Test %d: expected error log to contain `%s` in `%s`", i, test.expectedErr, runner.ReadErr())
		}
	}
}

func TestRunnerError(t *testing.T) {
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
		runner := New(&Config{
			Ctx:     context.Background(),
			Route:   test.route,
			Timeout: 5 * time.Second,
			Payload: test.payload,
		})

		if err := runner.Run(); err != nil {
			t.Fatalf("Test %d: error during Run() - %s", i, err)
		}

		if test.expectedStatus != runner.Status() {
			t.Fatalf("Test %d: expected result status to be `%s` but it was `%s`", i, test.expectedStatus, runner.Status())
		}

		if !bytes.Contains(runner.ReadOut(), []byte(test.expectedOut)) {
			t.Fatalf("Test %d: expected output log to contain `%s` in `%s`", i, test.expectedOut, runner.ReadOut())
		}

		if !bytes.Contains(runner.ReadErr(), []byte(test.expectedErr)) {
			t.Fatalf("Test %d: expected error log to contain `%s` in `%s`", i, test.expectedErr, runner.ReadErr())
		}
	}
}
