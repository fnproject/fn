package httpevent

import (
	"bytes"
	"encoding/json"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func withHeaders(r *http.Request, kvs ...string) *http.Request {
	if len(kvs)%2 != 0 {
		panic("need an even number of key/value pairs")
	}
	for i := 0; i < len(kvs); i += 2 {

		r.Header.Set(kvs[i], kvs[i+1])
	}
	return r
}

func TestRawHTTPReq(t *testing.T) {

	tcs := []struct {
		name string
		req  *http.Request
		json string
	}{
		{
			name: "calculates host based on request ",
			req:  httptest.NewRequest("GET", "/r/test?foo=bar", bytes.NewReader([]byte{})),
			json: `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPReq":{"method":"GET","requestURL":"http://example.com/r/test?foo=bar"}}}`,
		},
		{
			name: "get no body",
			req:  httptest.NewRequest("GET", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte{})),
			json: `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPReq":{"method":"GET","requestURL":"http://example.com/r/test?foo=bar"}}}`,
		},
		{
			name: "post json body",
			req:  withHeaders(httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`{"content":["foo"]}`))), "Content-Type", "application/json"),
			json: `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","contentType":"application/json","extensions":{"ioFnProjectHTTPReq":{"method":"POST","headers":{"Content-Type":["application/json"]},"requestURL":"http://example.com/r/test?foo=bar"}},"data":{"content":["foo"]}}`,
		},
		{
			name: "post string body",
			req:  withHeaders(httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`Foo bar`))), "Content-type", "application/wibble"),
			json: `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","contentType":"application/wibble","extensions":{"ioFnProjectHTTPReq":{"method":"POST","headers":{"Content-Type":["application/wibble"]},"requestURL":"http://example.com/r/test?foo=bar"}},"data":"Foo bar"}`,
		},
		{
			name: "post string body with Escaping ",
			req:  withHeaders(httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`Foo\n bar`))), "Content-type", "application/wibble"),
			json: `{
	"DefaultCloudEventVersion":"0.1",	"eventID":"EVENT",	"source":"http://example.com/r/test?foo=bar",	"eventType":"io.fnproject.httpRequest",	"eventTypeVersion":"0.1",	"eventTime":"1970-01-01T00:00:00.000Z",	"contentType":"application/wibble","extensions":{	"ioFnProjectHTTPReq":{	"method":"POST","headers":{"Content-Type":["application/wibble"]},"requestURL":"http://example.com/r/test?foo=bar"}},"data":"Foo\\n bar"}`,
		},
		{
			name: "No content type on incoming req with body ",
			req:  httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`Foo\n bar`))),
			json: `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","contentType":"application/octet-stream","extensions":{"ioFnProjectHTTPReq":{"method":"POST","requestURL":"http://example.com/r/test?foo=bar"}},"data":"Foo\\n bar"}`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			ce, err := FromHTTPRequest(tc.req, 4096)
			if err != nil {
				t.Fatal("Failed to produce event", err)
			}
			if ce.EventID == "" {
				t.Errorf("expecting event ID, got nothing ")
			}

			if time.Time(ce.EventTime).Before(start) || time.Time(ce.EventTime).After(time.Now()) {
				t.Errorf("Event time was not current, got %s", ce.EventTime.String())
			}
			// zero these out for checks
			ce.EventID = "EVENT"
			ce.EventTime = common.NewDateTime()

			j, err := json.Marshal(ce)

			if err != nil {
				t.Fatal("Failed to marshal event to JSON ", err)
			}
			if !jsonEqual(t, string(j), tc.json) {
				t.Errorf("Expected `%s`, got `%s`", tc.json, string(j))
			}

		})
	}
}

func TestUnsupportedReq(t *testing.T) {

	var overlengthEscapedArray = make([]byte, 4096)
	for i := range overlengthEscapedArray {
		overlengthEscapedArray[i] = '\\'
	}

	tcs := []struct {
		name  string
		req   *http.Request
		error models.Error
	}{
		{name: "put with invalid JSON ",
			req:   withHeaders(httptest.NewRequest("PUT", "/r/test?foo=bar", strings.NewReader("{")), "Content-Type", "application/json"),
			error: ErrInvalidJSONBody,
		},
		{name: "put non-unicode body",
			req:   withHeaders(httptest.NewRequest("PUT", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte{'\x80', '\x81'})), "Content-Type", "wibble/foo"),
			error: ErrUnsupportedBodyEncoding,
		},
		{name: "over-length-encoded-string",
			req:   withHeaders(httptest.NewRequest("PUT", "http://example.com/r/test?foo=bar", bytes.NewReader(overlengthEscapedArray)), "Content-Type", "wibble/foo"),
			error: ErrEncodedBodyTooLong,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromHTTPRequest(tc.req, 4096)

			if err != tc.error {
				t.Fatalf("expecting error %s , but got %s ", tc.error, err)
			}

		})
	}
}

func TestValidHTTPResp(t *testing.T) {

	tcs := []struct {
		name   string
		status int
		header map[string][]string
		body   []byte
		json   string
	}{
		{
			name:   "empty response",
			status: 200,
			json:   `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/","eventType":"io.fnproject.httpResponse", "eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPResp":{"status":200}}}`,
		},

		{
			name:   "non-200",
			status: 404,
			json:   `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/","eventType":"io.fnproject.httpResponse", "eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPResp":{"status":404}}}`,
		},

		{
			name:   "json-body-no-contentType",
			status: 200,
			body:   []byte(`{"test":"foo","bar":1,"baz":[101]}`),
			json:   `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/","eventType":"io.fnproject.httpResponse", "eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPResp":{"status":200}},"contentType":"application/octet-stream", "data":"{\"test\":\"foo\",\"bar\":1,\"baz\":[101]}"}`,
		},

		{
			name:   "json-body-json-contentType",
			status: 200,
			header: map[string][]string{"Content-type": {"application/json"}},
			body:   []byte(`{"test":"foo","bar":1,"baz":[101]}`),
			json:   `{"DefaultCloudEventVersion":"0.1","eventID":"EVENT","source":"http://example.com/","eventType":"io.fnproject.httpResponse", "eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPResp":{"status":200,"headers":{"Content-Type":["application/json"]}}},"contentType":"application/json", "data":{"test":"foo","bar":1,"baz":[101]}}`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()

			r := httptest.NewRecorder()
			for k, vs := range tc.header {
				for _, v := range vs {
					r.Header().Add(k, v)
				}
			}

			r.WriteHeader(tc.status)
			r.Write(tc.body)

			ce, err := FromHTTPResponse("http://example.com/", 4096, r.Result())

			if err != nil {
				t.Fatal("Failed to produce event", err)
			}
			if ce.EventID == "" {
				t.Errorf("expecting event ID, got nothing ")
			}

			if time.Time(ce.EventTime).Before(start) || time.Time(ce.EventTime).After(time.Now()) {
				t.Errorf("Event time was not current, got %s", ce.EventTime.String())
			}
			// zero these out for checks
			ce.EventID = "EVENT"
			ce.EventTime = common.NewDateTime()

			j, err := json.Marshal(ce)

			if err != nil {
				t.Fatal("Failed to marshal event to JSON ", err)
			}
			if !jsonEqual(t, string(j), tc.json) {
				t.Errorf("Expected `%s`, got `%s`", tc.json, string(j))
			}

		})
	}
}

func TestUnsupportedResp(t *testing.T) {

	tcs := []struct {
		name   string
		status int
		header map[string][]string
		body   []byte
		error  error
	}{
		{name: "response with empty JSON body",
			status: 200,
			header: map[string][]string{"Content-type": {"application/json"}},
			body:   []byte("{"),
			error:  ErrInvalidJSONBody,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			r := httptest.NewRecorder()
			for k, vs := range tc.header {
				for _, v := range vs {
					r.Header().Add(k, v)
				}
			}

			r.WriteHeader(tc.status)
			r.Write(tc.body)

			_, err := FromHTTPResponse("http://example.com/", 4096, r.Result())

			if err != tc.error {
				t.Fatalf("expecting error %s , but got %s ", tc.error, err)
			}

		})
	}
}

func jsonEqual(t *testing.T, j1, j2 string) bool {
	var jv1, jv2 interface{}

	err := json.Unmarshal([]byte(j1), &jv1)

	if err != nil {
		t.Fatalf("Failed to unmarshal `%s`: %s", j1, err)
	}

	err = json.Unmarshal([]byte(j2), &jv2)
	if err != nil {
		t.Fatalf("Failed to unmarshal `%s`: %s", j2, err)
	}

	return reflect.DeepEqual(jv1, jv2)

}
