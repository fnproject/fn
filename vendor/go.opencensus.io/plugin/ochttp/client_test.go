// Copyright 2018, OpenCensus Authors
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

package ochttp_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
)

const reqCount = 5

func TestClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Write([]byte("Hello, world!"))
	}))
	defer server.Close()

	for _, v := range ochttp.DefaultViews {
		v.Subscribe()
	}

	views := []string{
		"opencensus.io/http/client/request_count",
		"opencensus.io/http/client/latency",
		"opencensus.io/http/client/request_bytes",
		"opencensus.io/http/client/response_bytes",
	}
	for _, name := range views {
		v := view.Find(name)
		if v == nil {
			t.Errorf("view not found %q", name)
			continue
		}
	}

	var (
		w    sync.WaitGroup
		tr   ochttp.Transport
		errs = make(chan error, reqCount)
	)
	w.Add(reqCount)
	for i := 0; i < reqCount; i++ {
		go func() {
			defer w.Done()
			req, err := http.NewRequest("POST", server.URL, strings.NewReader("req-body"))
			if err != nil {
				errs <- fmt.Errorf("error creating request: %v", err)
			}
			resp, err := tr.RoundTrip(req)
			if err != nil {
				errs <- fmt.Errorf("response error: %v", err)
			}
			if err := resp.Body.Close(); err != nil {
				errs <- fmt.Errorf("error closing response body: %v", err)
			}
			if got, want := resp.StatusCode, 200; got != want {
				errs <- fmt.Errorf("resp.StatusCode=%d; wantCount %d", got, want)
			}
		}()
	}

	go func() {
		w.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, viewName := range views {
		v := view.Find(viewName)
		if v == nil {
			t.Errorf("view not found %q", viewName)
			continue
		}
		rows, err := v.RetrieveData()
		if err != nil {
			t.Error(err)
			continue
		}
		if got, want := len(rows), 1; got != want {
			t.Errorf("len(%q) = %d; want %d", viewName, got, want)
			continue
		}
		data := rows[0].Data
		var count int64
		switch data := data.(type) {
		case *view.CountData:
			count = *(*int64)(data)
		case *view.DistributionData:
			count = data.Count
		default:
			t.Errorf("don't know how to handle data type: %v", data)
		}
		if got := count; got != reqCount {
			t.Fatalf("%s = %d; want %d", viewName, got, reqCount)
		}
	}
}
