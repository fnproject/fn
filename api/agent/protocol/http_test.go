package protocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/event/httpevent"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func assertSameHeaders(t *testing.T, expect, got http.Header) {
	for h1, vs1 := range expect {
		vs2, ok := got[h1]
		if !ok {
			t.Errorf("expecting `%s` but was missing ", h1)
		} else {
			if !reflect.DeepEqual(vs1, vs2) {
				t.Errorf("expecting `%s` to be %#v but was %#v ", h1, vs1, vs2)
			}
		}
	}

	for h2, vs2 := range got {
		_, ok := expect[h2]
		if !ok {
			t.Errorf("expecting `%s` to be absent but was present with value  %#v ", h2, vs2)
		}
	}
}

func TestHTTPProtocolWrite(t *testing.T) {

	var Deadline = common.DateTime(time.Unix(1532443904, 0))
	var DeadlineStr = Deadline.String()
	var CallID = "CallID"
	var EvtID = "eventID"
	baseEvent := func(e *event.Event) *event.Event {
		e.SetDeadline(Deadline)
		e.SetCallID(CallID)
		e.EventID = EvtID
		e.EventTime = common.DateTime(time.Unix(1532443900, 0))
		e.CloudEventsVersion = "0.0"
		e.Source = "http://some-source"
		return e
	}

	tcs := []struct {
		name            string
		input           *event.Event
		expectedMethod  string
		expectedHeaders http.Header
		expectedBody    string
	}{
		{
			"event no body",
			baseEvent(&event.Event{}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"POST"},
				"Fn_request_url": {FakeSourceURL},
				"Fn_call_id":     {CallID},
				"Content-Length": {"0"},
			},
			"",
		},
		{
			"event JSON body",
			baseEvent(&event.Event{
				Data:        event.MustJSONBody(`{"hello": "world"}`),
				ContentType: "application/json",
			}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"POST"},
				"Fn_request_url": {FakeSourceURL},
				"Fn_call_id":     {CallID},
				"Content-Length": {"18"},
				"Content-Type":   {"application/json"},
			},
			`{"hello": "world"}`,
		},

		{
			"event non-JSON  body",
			baseEvent(&event.Event{
				Data:        event.MustStringBody(`hello world`),
				ContentType: "text/plain",
			}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"POST"},
				"Fn_request_url": {FakeSourceURL},
				"Fn_call_id":     {CallID},
				"Content-Length": {"11"},
				"Content-Type":   {"text/plain"},
			},
			`hello world`,
		},

		{
			"http event post no body",
			baseEvent(&event.Event{Extensions: map[string]event.ExtensionMessage{
				httpevent.ExtIoFnProjectHTTPReq: event.MustJSONExtMessage(&httpevent.HTTPReqExt{
					Method:     "POST",
					RequestURL: "/my-url",
				}),
			}}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"POST"},
				"Fn_request_url": {"/my-url"},
				"Fn_call_id":     {CallID},
				"Content-Length": {"0"},
			},
			"",
		},
		{
			"http event put json body",
			baseEvent(&event.Event{
				Data:        event.MustJSONBody(`{"hello": "world"}`),
				ContentType: "application/json",
				Extensions: map[string]event.ExtensionMessage{
					httpevent.ExtIoFnProjectHTTPReq: event.MustJSONExtMessage(&httpevent.HTTPReqExt{
						Method:     "PUT",
						RequestURL: "/my-url",
					}),
				}}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"PUT"},
				"Fn_request_url": {"/my-url"},
				"Fn_call_id":     {CallID},
				"Content-Length": {"18"},
				"Content-Type":   {"application/json"},
			},
			`{"hello": "world"}`,
		},

		{
			"http event put string body is unescaped JSON ",
			baseEvent(&event.Event{
				Data:        event.MustStringBody(`Hello World`),
				ContentType: "text/plain",
				Extensions: map[string]event.ExtensionMessage{
					httpevent.ExtIoFnProjectHTTPReq: event.MustJSONExtMessage(&httpevent.HTTPReqExt{
						Method:     "PUT",
						RequestURL: "/my-url",
					}),
				}}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"PUT"},
				"Fn_request_url": {"/my-url"},
				"Fn_call_id":     {CallID},
				"Content-Length": {"11"},
				"Content-Type":   {"text/plain"},
			},
			`Hello World`,
		},
		{
			"http event with headers",
			baseEvent(&event.Event{Extensions: map[string]event.ExtensionMessage{
				httpevent.ExtIoFnProjectHTTPReq: event.MustJSONExtMessage(&httpevent.HTTPReqExt{
					Method:     "POST",
					RequestURL: "/my-url",
					Headers: map[string][]string{
						"x-my-header": {"myval 1", "myval 2"}, // intentionally lowercased to check header canoicalisation
					},
				}),
			}}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"POST"},
				"Fn_request_url": {"/my-url"},
				"Fn_call_id":     {CallID},
				"Content-Length": {"0"},
				"X-My-Header":    {"myval 1", "myval 2"},
			},
			"",
		},
		{
			"user cant override fn headers",
			baseEvent(&event.Event{Extensions: map[string]event.ExtensionMessage{
				httpevent.ExtIoFnProjectHTTPReq: event.MustJSONExtMessage(&httpevent.HTTPReqExt{
					Method:     "POST",
					RequestURL: "/my-url",
					Headers: map[string][]string{
						"Fn_deadline":    {"Wibble"},
						"Fn_method":      {"Wibble"},
						"Fn_request_url": {"Wibble"},
						"Fn_call_id":     {"Wibble"},
					},
				}),
			}}),
			"POST",
			map[string][]string{
				"Fn_deadline":    {DeadlineStr},
				"Fn_method":      {"POST"},
				"Fn_request_url": {"/my-url"},
				"Fn_call_id":     {CallID},
				"Content-Length": {"0"},
			},
			"",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader("HTTP/1.1 200 OK\n\n")
			buf := &bytes.Buffer{}
			proto := httpProtocol{"/mysource", 1024, buf, reader}

			_, err := proto.Dispatch(context.Background(), tc.input)
			if err != nil {
				t.Fatalf("Invalid dispatch %s", err)
			}

			req, err := http.ReadRequest(bufio.NewReader(buf))
			if err != nil {
				t.Fatal("invalid HTTP request sent to container ", err)
			}

			if req.Method != tc.expectedMethod {
				t.Errorf("invalid method, expected '%s', got '%s'", tc.expectedMethod, req.Method)
			}

			req.Header.Del("User-agent")
			assertSameHeaders(t, tc.expectedHeaders, req.Header)
			bodyBuf := &bytes.Buffer{}
			bodyBuf.ReadFrom(req.Body)
			if !bytes.Equal(bodyBuf.Bytes(), []byte(tc.expectedBody)) {
				t.Errorf("invalid body, expected \n'%s', got \n'%s'", tc.expectedBody, string(bodyBuf.Bytes()))
			}
		})
	}
}

func TestHTTPProtocolRead(t *testing.T) {
	dateTime := common.DateTime(time.Unix(1532443900, 0))
	var EvtID = "eventID"
	baseInEvent := func(e *event.Event) *event.Event {
		e.EventID = EvtID
		e.EventTime = dateTime
		e.CloudEventsVersion = "0.1"
		e.Source = "/mysource"
		e.SetCallID("CallID")
		e.SetDeadline(dateTime)
		return e
	}

	baseOutEvent := func(e *event.Event) *event.Event {
		e.EventID = EvtID
		e.EventTime = dateTime
		e.CloudEventsVersion = "0.1"
		e.Source = "/mysource"
		e.EventType = httpevent.EventTypeHTTPResp
		e.SetCallID("CallID")
		return e
	}

	tcs := []struct {
		name          string
		responseHTTP  string
		expectedEvent *event.Event
	}{
		{
			"emptyResponse",
			"HTTP/1.1 200 OK\n\n",
			baseOutEvent(&event.Event{Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200})}}),
		},
		{
			"body no content type json value",
			"HTTP/1.1 200 OK\nContent-Length:17\n\n{\"hello\":\"world\"}",
			baseOutEvent(&event.Event{Data: event.MustJSONBody(`{"hello":"world"}`), ContentType: "application/json", Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200, Headers: map[string][]string{"Content-Length": {"17"}}})}}),
		},
		{
			"body with content type string body",
			"HTTP/1.1 200 OK\nContent-Type:text/plain\nContent-Length:12\n\nHello\n World",
			baseOutEvent(&event.Event{Data: event.MustStringBody("Hello\n World"), ContentType: "text/plain", Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200, Headers: map[string][]string{"Content-Length": {"12"}, "Content-Type": {"text/plain"}}})}}),
		},
		//{
		//	"body with content type json  body ",
		//	`{"body": "hello world\n from fn","content_type":"application/json"}`,
		//	baseOutEvent(&event.Event{Data: event.MustJSONBody("\"hello world\\n from fn\""), ContentType: "application/json", Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200})}}),
		//},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.responseHTTP)
			proto := httpProtocol{"/mysource", 1024, ioutil.Discard, reader}

			outEvent, err := proto.Dispatch(context.Background(), baseInEvent(&event.Event{}))

			if err != nil {
				t.Fatalf("Invalid dispatch %s", err)
			}
			if outEvent.EventID == "" {
				t.Fatal("Expected inputs ID got nil ")
			}
			outEvent.EventID = EvtID

			if outEvent.EventTime == common.NewDateTime() {
				t.Fatalf("No inputs time specified")
			}
			outEvent.EventTime = dateTime

			ceJson, err := json.Marshal(outEvent)

			if err != nil {
				t.Fatal("invalid inputs, can't marshal raw", err)
			}

			expectedJSON, err := json.Marshal(tc.expectedEvent)
			if err != nil {
				t.Fatal("Invalid expected, can't marshal", err)
			}
			if !jsonEqual(t, string(expectedJSON), string(ceJson)) {
				t.Errorf("expected inputs to be \n`%s`, was \n`%s", string(expectedJSON), string(ceJson))
			}
		})
	}
}
