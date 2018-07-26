package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/event/httpevent"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func TestJSONProtocolWrite(t *testing.T) {

	var Deadline = common.DateTime(time.Unix(1532443904, 0))
	//var DeadlineStr = "2018-07-24T14:51:44+00:00.000Z"
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
		name         string
		input        *event.Event
		expectedJson string
	}{{
		"string-non-http",
		baseEvent(&event.Event{Data: event.MustStringBody("hello world"), ContentType: "text/plain"}),
		`{"body":"hello world","content_type":"text/plain","deadline":"2018-07-24T15:51:44.000+01:00","call_id" :"CallID" ,"protocol":{"type":"http","method":"GET","request_url":"http://fnproject.io/s/non-http-inputs","headers":{}}}`,
	}, {
		"http",
		baseEvent(&event.Event{Source: "http://some-source",
			Data:        event.MustStringBody("hello world"),
			ContentType: "text/plain",
			Extensions: map[string]event.ExtensionMessage{
				httpevent.ExtIoFnProjectHTTPReq: event.MustJSONExtMessage(&httpevent.HTTPReqExt{
					Method:     "PUT",
					RequestURL: "http://my_url/foo",
					Headers: map[string][]string{
						"X-MyHeader": {"myval1", "myval2"},
					},
				})}}),
		`{"call_id":"CallID","deadline":"2018-07-24T15:51:44.000+01:00","body":"hello world","content_type":"text/plain","protocol":{"type":"http","method":"PUT","request_url":"http://my_url/foo","headers":{"X-MyHeader":["myval1","myval2"]}}}`,
	}, {
		"empty-body",
		baseEvent(&event.Event{}),
		`{"call_id":"CallID","deadline":"2018-07-24T15:51:44.000+01:00","body":null,"content_type":"","protocol":{"type":"http","method":"GET","request_url":"http://fnproject.io/s/non-http-inputs","headers":{}}}`,
	}, {
		"json-body",
		baseEvent(&event.Event{ContentType: "application/json", Data: event.MustJSONBody(`{"some_body":[1,2,3]}`)}),
		`{"body":{"some_body":[1,2,3]},"content_type":"application/json","deadline":"2018-07-24T15:51:44.000+01:00","call_id" :"CallID" ,"protocol":{"type":"http","method":"GET","request_url":"http://fnproject.io/s/non-http-inputs","headers":{}}}`,
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			wbuf := &bytes.Buffer{}
			reader := strings.NewReader(`{"body": "ok"}`)
			proto := JSONProtocol{"/mysource", 1024, wbuf, reader}

			_, err := proto.Dispatch(context.Background(), tc.input)
			if err != nil {
				t.Fatalf("Invalid dispatch %s", err)
			}

			if !jsonEqual(t, tc.expectedJson, string(wbuf.Bytes())) {
				t.Errorf("expected body to be \n `%s`, was \n`%s", tc.expectedJson, string(wbuf.Bytes()))
			}
		})
	}
}

func TestJSONProtocolRead(t *testing.T) {
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
		responseJson  string
		expectedEvent *event.Event
	}{{
		"emptyResponse",
		"{}",
		baseOutEvent(&event.Event{Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200})}}),
	}, {
		"body no content type json value",
		`{"body": "hello world\n from fn"}`,
		baseOutEvent(&event.Event{Data: event.MustStringBody("hello world\n from fn"), ContentType: "application/json", Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200})}}),
	}, {
		"body with content type string body",
		`{"body": "hello world\n from fn","content_type":"text/plain"}`,
		baseOutEvent(&event.Event{Data: event.MustStringBody("hello world\n from fn"), ContentType: "text/plain", Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200})}}),
	}, {
		"body with content type json  body ",
		`{"body": "hello world\n from fn","content_type":"application/json"}`,
		baseOutEvent(&event.Event{Data: event.MustJSONBody("\"hello world\\n from fn\""), ContentType: "application/json", Extensions: map[string]event.ExtensionMessage{httpevent.ExtIoFnProjectHTTPResp: event.MustJSONExtMessage(&httpevent.HTTPRespExt{Status: 200})}}),
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.responseJson)
			proto := JSONProtocol{"/mysource", 1024, ioutil.Discard, reader}

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

func TestInvalidContentFromContainer(t *testing.T) {
	dateTime := common.DateTime(time.Unix(1532443900, 0))
	var EvtID = "eventID"
	inEvent := &event.Event{
		EventID:            EvtID,
		EventTime:          dateTime,
		CloudEventsVersion: "0.1",
		Source:             "/mysource",
	}
	inEvent.SetCallID("CallID")
	inEvent.SetDeadline(dateTime)

	longString := strings.Repeat("a", 1025)
	tcs := []struct {
		name   string
		reader io.Reader
		err    error
	}{
		{"excess json ",
			strings.NewReader("{} {}"),
			ErrExcessData,
		},
		{
			"excess whitespace ok ",
			strings.NewReader("{}  "),
			nil,
		},

		{
			"partial content",
			strings.NewReader("{  "),
			ErrInvalidContentFromContainer,
		},

		{
			"Too much input",
			strings.NewReader(`{"body":"` + longString + `"}`),
			ErrContainerResponseTooLarge,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			proto := JSONProtocol{"/mysource", 1024, ioutil.Discard, tc.reader}
			_, err := proto.Dispatch(context.Background(), inEvent)
			if err != tc.err {
				t.Fatalf("Expected error '%s' got '%s' ", tc.err, err)
			}
		})
	}

}
