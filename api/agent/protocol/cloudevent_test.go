package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
	"time"
)

func mustRender(t *testing.T, evt *event.Event) []byte {
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatal("Failed to render event to JSON", err)
	}
	return b
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

func TestCloudEventWriteMessages(t *testing.T) {

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

	outEvent := &event.Event{
		EventID:            "outEvent",
		EventTime:          common.NewDateTime(),
		CloudEventsVersion: "0.1",
		Source:             "/mysource",
		ContentType:        "text/plain",
		Data:               event.MustStringBody("hello"),
	}
	evtBytes, _ := json.Marshal(outEvent)

	wbuf := &bytes.Buffer{}

	proto := cloudEventProtocol{"/evsource", 1024, wbuf, bytes.NewReader(evtBytes)}

	_, err := proto.Dispatch(context.Background(), inEvent)

	if err != nil {
		t.Fatal("got error ", err)
	}

	var evt *event.Event
	err = json.Unmarshal(wbuf.Bytes(), &evt)
	if err != nil {
		t.Fatal("Failed to read written event", err)
	}

	if !reflect.DeepEqual(evt, inEvent) {
		t.Errorf("Writen evvent was not the same as input event wanted:\n%s\ngot\n%s", string(evtBytes), string(wbuf.Bytes()))
	}
}

func TestCloudEventReadValid(t *testing.T) {
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

	outEvent := &event.Event{
		EventID:            "OutID",
		EventTime:          common.NewDateTime(),
		CloudEventsVersion: "0.1",
		Data:               event.MustStringBody("data"),
		ContentType:        "text/plain",
		Source:             "/mysource",
	}
	outEvent.SetExtension("myext", "val")

	outBytes, _ := json.Marshal(outEvent)

	proto := cloudEventProtocol{"/fnsource", 1024, ioutil.Discard, bytes.NewReader(outBytes)}

	gotEvt, err := proto.Dispatch(context.Background(), inEvent)

	if err != nil {
		t.Fatal("Failed to submit event", err)
	}

	if gotEvt.EventID != "CallID" {
		t.Errorf("expected call ID as event ID  got %s", gotEvt.EventID)
	}
	if outEvent.EventTime == gotEvt.EventTime {
		t.Errorf("expected event time to be overridden")
	}
	if gotEvt.Source != "/fnsource" {
		t.Errorf("expected event source to be overwritten with fn source")
	}

	if !bytes.Equal(gotEvt.Data, []byte("\"data\"")) {
		t.Errorf("unexpected body %s", string(gotEvt.Data))
	}
	if gotEvt.ContentType != "text/plain" {
		t.Errorf("unexpected content type %s", gotEvt.ContentType)
	}
	var ext string
	err = gotEvt.ReadExtension("myext", &ext)
	if err != nil {
		t.Errorf("bad extension on response")
	}
	if ext != "val" {
		t.Errorf("bad unexpected extension value %s", ext)
	}
}

func TestCloudEventInvalidResponses(t *testing.T) {
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

	goodEvent := &event.Event{
		EventID:            "OutID",
		EventTime:          common.NewDateTime(),
		CloudEventsVersion: "0.1",
		Data:               event.MustStringBody("data"),
		ContentType:        "text/plain",
		Source:             "/mysource",
	}
	goodEvent.SetExtension("myext", "val")

	longString := strings.Repeat("a", 1025)
	longEvent := *goodEvent
	longEvent.Data = event.MustStringBody(longString)

	badContentTypeEvent := *goodEvent
	badContentTypeEvent.ContentType = ""

	tcs := []struct {
		name   string
		reader io.Reader
		err    error
	}{
		{"excess json ",
			strings.NewReader(string(mustRender(t, goodEvent)) + " {}"),
			ErrExcessData,
		},
		{
			"excess whitespace ok ",
			strings.NewReader(string(mustRender(t, goodEvent)) + " \t\n  "),
			nil,
		},

		{
			"partial content",
			strings.NewReader("{  "),
			ErrInvalidContentFromContainer,
		},

		{
			"Too much input",
			bytes.NewReader(mustRender(t, &longEvent)),
			ErrContainerResponseTooLarge,
		},
		{
			"Bad content type",
			bytes.NewReader(mustRender(t, &badContentTypeEvent)),
			ErrMissingResponseContentType,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			proto := cloudEventProtocol{"/mysource", 1024, ioutil.Discard, tc.reader}
			_, err := proto.Dispatch(context.Background(), inEvent)
			if err != tc.err {
				t.Fatalf("Expected error '%s' got '%s' ", tc.err, err)
			}
		})
	}

}
