package event

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
			json: `{"cloudEventsVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPReq":{"method":"GET","requestURL":"http://example.com/r/test?foo=bar"}}}`,
		},
		{
			name: "get no body",
			req:  httptest.NewRequest("GET", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte{})),
			json: `{"cloudEventsVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPReq":{"method":"GET","requestURL":"http://example.com/r/test?foo=bar"}}}`,
		},
		{
			name: "post json body",
			req:  withHeaders(httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`{"content":["foo"]}`))), "Content-Type", "application/json"),
			json: `{"cloudEventsVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPReq":{"method":"POST","headers":{"Content-Type":["application/json"]},"requestURL":"http://example.com/r/test?foo=bar"}},"data":{"content":["foo"]}}`,
		},
		{
			name: "post string body",
			req:  withHeaders(httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`Foo bar`))), "Content-type", "application/wibble"),
			json: `{"cloudEventsVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPReq":{"method":"POST","headers":{"Content-Type":["application/wibble"]},"requestURL":"http://example.com/r/test?foo=bar"}},"data":"Foo bar"}`,
		},
		{
			name: "post string body with Escaping ",
			req:  withHeaders(httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`Foo\n bar`))), "Content-type", "application/wibble"),
			json: `{
	"cloudEventsVersion":"0.1",	"eventID":"EVENT",	"source":"http://example.com/r/test?foo=bar",	"eventType":"io.fnproject.httpRequest",	"eventTypeVersion":"0.1",	"eventTime":"1970-01-01T00:00:00.000Z",	"extensions":{	"ioFnProjectHTTPReq":{	"method":"POST","headers":{"Content-Type":["application/wibble"]},"requestURL":"http://example.com/r/test?foo=bar"}},"data":"Foo\\n bar"}`,
		},
		{
			name: "No content type on incoming req with body ",
			req:  httptest.NewRequest("POST", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte(`Foo\n bar`))),
			json: `{"cloudEventsVersion":"0.1","eventID":"EVENT","source":"http://example.com/r/test?foo=bar","eventType":"io.fnproject.httpRequest","eventTypeVersion":"0.1","eventTime":"1970-01-01T00:00:00.000Z","extensions":{"ioFnProjectHTTPReq":{"method":"POST","requestURL":"http://example.com/r/test?foo=bar"}},"data":"Foo\\n bar"}`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			ce, err := FromHTTPTrigger(tc.req)
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

	tcs := []struct {
		name  string
		req   *http.Request
		error models.APIError
	}{
		{name: "put with invalid JSON ",
			req:   withHeaders(httptest.NewRequest("PUT", "/r/test?foo=bar", strings.NewReader("{")), "Content-Type", "application/json"),
			error: ErrInvalidJSONBody,
		},
		{name: "put non-unicode body",
			req:   withHeaders(httptest.NewRequest("PUT", "http://example.com/r/test?foo=bar", bytes.NewReader([]byte{'\x80', '\x81'})), "Content-Type", "wibble/foo"),
			error: ErrUnsupportedBodyEncoding,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromHTTPTrigger(tc.req)

			if err != tc.error {
				t.Fatal("expecting error %s , but got %s ", tc.error, err)
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
