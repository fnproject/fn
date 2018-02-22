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

package ochttp

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.opencensus.io/trace"
)

type testTransport struct {
	ch chan *http.Request
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.ch <- req
	return nil, errors.New("noop")
}

type testPropagator struct{}

func (t testPropagator) SpanContextFromRequest(req *http.Request) (sc trace.SpanContext, ok bool) {
	header := req.Header.Get("trace")
	buf, err := hex.DecodeString(header)
	if err != nil {
		log.Fatalf("Cannot decode trace header: %q", header)
	}
	r := bytes.NewReader(buf)
	r.Read(sc.TraceID[:])
	r.Read(sc.SpanID[:])
	opts, err := r.ReadByte()
	if err != nil {
		log.Fatalf("Cannot read trace options from trace header: %q", header)
	}
	sc.TraceOptions = trace.TraceOptions(opts)
	return sc, true
}

func (t testPropagator) SpanContextToRequest(sc trace.SpanContext, req *http.Request) {
	var buf bytes.Buffer
	buf.Write(sc.TraceID[:])
	buf.Write(sc.SpanID[:])
	buf.WriteByte(byte(sc.TraceOptions))
	req.Header.Set("trace", hex.EncodeToString(buf.Bytes()))
}

func TestTransport_RoundTrip(t *testing.T) {
	parent := trace.NewSpan("parent", nil, trace.StartOptions{})
	tests := []struct {
		name   string
		parent *trace.Span
	}{
		{
			name:   "no parent",
			parent: nil,
		},
		{
			name:   "parent",
			parent: parent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := &testTransport{ch: make(chan *http.Request, 1)}
			rt := &traceTransport{format: &testPropagator{}}
			rt.base = transport

			req, _ := http.NewRequest("GET", "http://foo.com", nil)
			if tt.parent != nil {
				req = req.WithContext(trace.WithSpan(req.Context(), tt.parent))
			}
			rt.RoundTrip(req)

			req = <-transport.ch
			span := trace.FromContext(req.Context())

			if header := req.Header.Get("trace"); header == "" {
				t.Fatalf("Trace header = empty; want valid trace header")
			}
			if span == nil {
				t.Fatalf("Got no spans in req context; want one")
			}
			if tt.parent != nil {
				if got, want := span.SpanContext().TraceID, tt.parent.SpanContext().TraceID; got != want {
					t.Errorf("span.SpanContext().TraceID=%v; want %v", got, want)
				}
			}
		})
	}
}

func TestHandler(t *testing.T) {
	// TODO(#431): remove SetDefaultSampler
	trace.SetDefaultSampler(trace.ProbabilitySampler(0.0))

	traceID := [16]byte{16, 84, 69, 170, 120, 67, 188, 139, 242, 6, 177, 32, 0, 16, 0, 0}
	tests := []struct {
		header           string
		wantTraceID      trace.TraceID
		wantTraceOptions trace.TraceOptions
	}{
		{
			header:           "105445aa7843bc8bf206b12000100000000000000000000000",
			wantTraceID:      traceID,
			wantTraceOptions: trace.TraceOptions(0),
		},
		{
			header:           "105445aa7843bc8bf206b12000100000000000000000000001",
			wantTraceID:      traceID,
			wantTraceOptions: trace.TraceOptions(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			propagator := &testPropagator{}

			handler := &Handler{
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					span := trace.FromContext(r.Context())
					sc := span.SpanContext()
					if got, want := sc.TraceID, tt.wantTraceID; got != want {
						t.Errorf("TraceID = %q; want %q", got, want)
					}
					if got, want := sc.TraceOptions, tt.wantTraceOptions; got != want {
						t.Errorf("TraceOptions = %v; want %v", got, want)
					}
				}),
			}
			handler.Propagation = propagator
			req, _ := http.NewRequest("GET", "http://foo.com", nil)
			req.Header.Add("trace", tt.header)
			handler.ServeHTTP(nil, req)
		})
	}
}

var _ http.RoundTripper = (*traceTransport)(nil)

type collector []*trace.SpanData

func (c *collector) ExportSpan(s *trace.SpanData) {
	*c = append(*c, s)
}

func TestEndToEnd(t *testing.T) {
	var spans collector
	trace.RegisterExporter(&spans)
	defer trace.UnregisterExporter(&spans)

	span := trace.NewSpan(
		"top-level",
		nil,
		trace.StartOptions{
			Sampler: trace.AlwaysSample(),
		})
	ctx := trace.WithSpan(context.Background(), span)

	serverDone := make(chan struct{})
	serverReturn := make(chan time.Time)
	url := serveHTTP(serverDone, serverReturn)

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/example/url/path?qparam=val", url),
		strings.NewReader("expected-request-body"))
	if err != nil {
		t.Fatalf("unexpected error %#v", err)
	}
	req = req.WithContext(ctx)

	rt := &traceTransport{format: defaultFormat, base: http.DefaultTransport}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected stats: %d", resp.StatusCode)
	}

	serverReturn <- time.Now().Add(time.Millisecond)

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("unexpected read error: %#v", err)
	}
	if string(respBody) != "expected-response" {
		t.Fatalf("unexpected response: %s", string(respBody))
	}

	resp.Body.Close()

	<-serverDone
	trace.UnregisterExporter(&spans)

	if got, want := len(spans), 2; got != want {
		t.Fatalf("len(%#v) = %d; want %d", spans, got, want)
	}

	var client, server *trace.SpanData
	for _, sp := range spans {
		if strings.HasPrefix(sp.Name, "Sent.") {
			client = sp
			serverHostport := req.URL.Hostname() + ":" + req.URL.Port()
			if got, want := client.Name, "Sent."+serverHostport+"/example/url/path"; got != want {
				t.Errorf("Span name: %q; want %q", got, want)
			}
		} else if strings.HasPrefix(sp.Name, "Recv.") {
			server = sp
			if got, want := server.Name, "Recv./example/url/path"; got != want {
				t.Errorf("Span name: %q; want %q", got, want)
			}
		}
	}

	if server == nil || client == nil {
		t.Fatalf("server or client span missing")
	}
	if server.TraceID != client.TraceID {
		t.Errorf("TraceID does not match: server.TraceID=%q client.TraceID=%q", server.TraceID, client.TraceID)
	}
	if server.StartTime.Before(client.StartTime) {
		t.Errorf("server span starts before client span")
	}
	if server.EndTime.After(client.EndTime) {
		t.Errorf("client span ends before server span")
	}
	if !server.HasRemoteParent {
		t.Errorf("server span should have remote parent")
	}
	if server.ParentSpanID != client.SpanID {
		t.Errorf("server span should have client span as parent")
	}
}

func serveHTTP(done chan struct{}, wait chan time.Time) string {
	handler := &Handler{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()

			// simulate a slow-responding server
			sleepUntil := <-wait
			for time.Now().Before(sleepUntil) {
				time.Sleep(sleepUntil.Sub(time.Now()))
			}

			io.WriteString(w, "expected-response")
			close(done)
		}),
	}

	server := httptest.NewServer(handler)
	go func() {
		<-done
		server.Close()
	}()
	return server.URL
}

func TestSpanNameFromURL(t *testing.T) {
	tests := []struct {
		prefix string
		u      string
		want   string
	}{
		{
			prefix: "Sent",
			u:      "http://localhost:80/hello?q=a",
			want:   "Sent.localhost/hello",
		},
		{
			prefix: "Recv",
			u:      "https://localhost:443/a",
			want:   "Recv.localhost/a",
		},
		{
			prefix: "Recv",
			u:      "https://example.com:7654/a",
			want:   "Recv.example.com:7654/a",
		},
		{
			prefix: "Sent",
			u:      "/a/b?q=c",
			want:   "Sent./a/b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.prefix+"-"+tt.u, func(t *testing.T) {
			u, err := url.Parse(tt.u)
			if err != nil {
				t.Errorf("url.Parse() = %v", err)
			}
			if got := spanNameFromURL(tt.prefix, u); got != tt.want {
				t.Errorf("spanNameFromURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestAttributes(t *testing.T) {
	tests := []struct {
		name      string
		makeReq   func() *http.Request
		wantAttrs []trace.Attribute
	}{
		{
			name: "GET example.com/hello",
			makeReq: func() *http.Request {
				req, _ := http.NewRequest("GET", "http://example.com/hello", nil)
				req.Header.Add("User-Agent", "ua")
				req.ContentLength = 128
				return req
			},
			wantAttrs: []trace.Attribute{
				trace.StringAttribute{Key: URLAttribute, Value: "http://example.com/hello"},
				trace.StringAttribute{Key: HostAttribute, Value: "example.com"},
				trace.StringAttribute{Key: MethodAttribute, Value: "GET"},
				trace.StringAttribute{Key: PathAttribute, Value: "/hello"},
				trace.StringAttribute{Key: UserAgentAttribute, Value: "ua"},
				trace.Int64Attribute{Key: RequestSizeAttribute, Value: 128},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.makeReq()
			attrs := requestAttrs(req)

			if got, want := attrs, tt.wantAttrs; !reflect.DeepEqual(got, want) {
				t.Errorf("Request attributes = %#v; want %#v", got, want)
			}
		})
	}
}

func TestResponseAttributes(t *testing.T) {
	tests := []struct {
		name      string
		resp      *http.Response
		wantAttrs []trace.Attribute
	}{
		{
			name: "non-zero HTTP 200 response",
			resp: &http.Response{StatusCode: 200, ContentLength: 128},
			wantAttrs: []trace.Attribute{
				trace.Int64Attribute{Key: ResponseSizeAttribute, Value: 128},
				trace.Int64Attribute{Key: StatusCodeAttribute, Value: 200},
			},
		},
		{
			name: "zero HTTP 500 response",
			resp: &http.Response{StatusCode: 500, ContentLength: 0},
			wantAttrs: []trace.Attribute{
				trace.Int64Attribute{Key: ResponseSizeAttribute, Value: 0},
				trace.Int64Attribute{Key: StatusCodeAttribute, Value: 500},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := responseAttrs(tt.resp)
			if got, want := attrs, tt.wantAttrs; !reflect.DeepEqual(got, want) {
				t.Errorf("Response attributes = %#v; want %#v", got, want)
			}
		})
	}
}
