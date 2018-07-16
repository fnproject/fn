package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fnproject/fn/api/common"
)

// implements CallInfo, modify as needed
type testCall struct {
	cloud       bool
	contentType string
	input       io.Reader
}

func (t *testCall) IsCloudEvent() bool  { return t.cloud }
func (t *testCall) CallID() string      { return "foo" }
func (t *testCall) ContentType() string { return t.contentType }
func (t *testCall) Input() io.Reader    { return t.input }
func (t *testCall) Deadline() common.DateTime {
	return common.DateTime(time.Now().Add(30 * time.Second))
}
func (t *testCall) CallType() string             { return "sync" }
func (t *testCall) ProtocolType() string         { return "http" }
func (t *testCall) Request() *http.Request       { return nil } // unused here
func (t *testCall) Method() string               { return "GET" }
func (t *testCall) RequestURL() string           { return "http://example.com/r/yo/dawg" }
func (t *testCall) Headers() map[string][]string { return map[string][]string{} }

func TestJSONMap(t *testing.T) {
	in := strings.NewReader(`{"yo":"dawg"}`)

	var ib, ob bytes.Buffer
	cep := &cloudEventProtocol{
		in:  &ib,
		out: &ob,
	}

	tc := &testCall{false, "application/json; charset=utf-8", in}

	err := cep.writeJSONToContainer(tc)
	if err != nil {
		t.Fatal(err)
	}

	var oce CloudEvent
	err = json.NewDecoder(&ib).Decode(&oce)
	if err != nil {
		t.Fatal(err)
	}

	mappo, ok := oce.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data field should be map[string]interface{}: %T", oce.Data)
	}

	v, ok := mappo["yo"].(string)
	if v != "dawg" {
		t.Fatal("value in map is wrong", v)
	}
}

func TestJSONNotMap(t *testing.T) {
	// we accept all json values here https://tools.ietf.org/html/rfc7159#section-3
	in := strings.NewReader(`true`)

	var ib, ob bytes.Buffer
	cep := &cloudEventProtocol{
		in:  &ib,
		out: &ob,
	}

	tc := &testCall{false, "application/json", in}

	err := cep.writeJSONToContainer(tc)
	if err != nil {
		t.Fatal(err)
	}

	var oce CloudEvent
	err = json.NewDecoder(&ib).Decode(&oce)
	if err != nil {
		t.Fatal(err)
	}

	boolo, ok := oce.Data.(bool)
	if !ok {
		t.Fatalf("data field should be bool: %T", oce.Data)
	}

	if !boolo {
		t.Fatal("bool should be true", boolo)
	}
}
