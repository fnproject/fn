// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runner

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/iron-io/functions/api/models"
)

func TestRunnerHello(t *testing.T) {
	buf := setLogBuffer()
	runner, err := New(NewMetricLogger())
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
			ID:      fmt.Sprintf("hello-%d-%d", i, time.Now().Unix()),
			Image:   test.route.Image,
			Timeout: 5 * time.Second,
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
	t.Skip()
	buf := setLogBuffer()
	runner, err := New(NewMetricLogger())
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
			ID:      fmt.Sprintf("err-%d-%d", i, time.Now().Unix()),
			Image:   test.route.Image,
			Timeout: 5 * time.Second,
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
