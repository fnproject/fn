package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
)

type RequestData struct {
	A string `json:"a"`
}

func setupRequest(data interface{}) (*callInfoImpl, context.CancelFunc) {
	req := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme:   "http",
			Host:     "localhost:8080",
			Path:     "/v1/apps",
			RawQuery: "something=something&etc=etc",
		},
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Host":         []string{"localhost:8080"},
			"User-Agent":   []string{"curl/7.51.0"},
			"Content-Type": []string{"application/json"},
		},
		Host: "localhost:8080",
	}
	var buf bytes.Buffer
	if data != nil {
		_ = json.NewEncoder(&buf).Encode(data)
	}
	req.Body = ioutil.NopCloser(&buf)

	call := &models.Call{
		Type:    "sync",
		Method:  req.Method,
		Headers: req.Header,
		Payload: buf.String(),
		URL:     req.URL.String(),
	}

	ctx, cancel := context.WithTimeout(req.Context(), 1*time.Second)
	deadline, _ := ctx.Deadline()
	ci := &callInfoImpl{call: call, deadline: common.DateTime(deadline)}
	return ci, cancel
}

func TestJSONProtocolwriteJSONInputRequestBasicFields(t *testing.T) {
	ci, cancel := setupRequest(nil)
	defer cancel()
	r, w := io.Pipe()
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.writeJSONToContainer(ci)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := &jsonIn{}
	bb := new(bytes.Buffer)

	_, err := bb.ReadFrom(r)
	if err != nil {
		t.Error(err.Error())
	}
	err = json.Unmarshal(bb.Bytes(), incomingReq)
	if err != nil {
		t.Error(err.Error())
	}
	if incomingReq.CallID != ci.CallID() {
		t.Errorf("Request CallID assertion mismatch: expected: %s, got %s",
			ci.CallID(), incomingReq.CallID)
	}
	if incomingReq.ContentType != ci.ContentType() {
		t.Errorf("Request ContentType assertion mismatch: expected: %s, got %s",
			ci.ContentType(), incomingReq.ContentType)
	}
	if incomingReq.Deadline != ci.Deadline().String() {
		t.Errorf("Request Deadline assertion mismatch: expected: %s, got %s",
			ci.Deadline(), incomingReq.Deadline)
	}
}

func TestJSONProtocolwriteJSONInputRequestWithData(t *testing.T) {
	rDataBefore := RequestData{A: "a"}
	ci, cancel := setupRequest(rDataBefore)
	defer cancel()
	r, w := io.Pipe()
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.writeJSONToContainer(ci)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := &jsonIn{}
	bb := new(bytes.Buffer)

	_, err := bb.ReadFrom(r)
	if err != nil {
		t.Error(err.Error())
	}
	err = json.Unmarshal(bb.Bytes(), incomingReq)
	if err != nil {
		t.Error(err.Error())
	}
	rDataAfter := new(RequestData)
	err = json.Unmarshal([]byte(incomingReq.Body), &rDataAfter)
	if err != nil {
		t.Error(err.Error())
	}
	if rDataBefore.A != rDataAfter.A {
		t.Errorf("Request data assertion mismatch: expected: %s, got %s",
			rDataBefore.A, rDataAfter.A)
	}
	if incomingReq.Protocol.Type != ci.ProtocolType() {
		t.Errorf("Call protocol type assertion mismatch: expected: %s, got %s",
			ci.ProtocolType(), incomingReq.Protocol.Type)
	}
	if incomingReq.Protocol.Method != ci.Method() {
		t.Errorf("Call protocol method assertion mismatch: expected: %s, got %s",
			ci.Method(), incomingReq.Protocol.Method)
	}
	if incomingReq.Protocol.RequestURL != ci.RequestURL() {
		t.Errorf("Call protocol request URL assertion mismatch: expected: %s, got %s",
			ci.RequestURL(), incomingReq.Protocol.RequestURL)
	}
}

func TestJSONProtocolwriteJSONInputRequestWithoutData(t *testing.T) {
	ci, cancel := setupRequest(nil)
	defer cancel()
	r, w := io.Pipe()
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.writeJSONToContainer(ci)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := &jsonIn{}
	bb := new(bytes.Buffer)

	_, err := bb.ReadFrom(r)
	if err != nil {
		t.Error(err.Error())
	}
	err = json.Unmarshal(bb.Bytes(), incomingReq)
	if err != nil {
		t.Error(err.Error())
	}
	if incomingReq.Body != "" {
		t.Errorf("Request body assertion mismatch: expected: %s, got %s",
			"<empty-string>", incomingReq.Body)
	}
	if !models.Headers(ci.call.Headers).Equals(models.Headers(incomingReq.Protocol.Headers)) {
		t.Errorf("Request headers assertion mismatch: expected: %s, got %s",
			ci.call.Headers, incomingReq.Protocol.Headers)
	}
	if incomingReq.Protocol.Type != ci.ProtocolType() {
		t.Errorf("Call protocol type assertion mismatch: expected: %s, got %s",
			ci.ProtocolType(), incomingReq.Protocol.Type)
	}
	if incomingReq.Protocol.Method != ci.Method() {
		t.Errorf("Call protocol method assertion mismatch: expected: %s, got %s",
			ci.Method(), incomingReq.Protocol.Method)
	}
	if incomingReq.Protocol.RequestURL != ci.RequestURL() {
		t.Errorf("Call protocol request URL assertion mismatch: expected: %s, got %s",
			ci.RequestURL(), incomingReq.Protocol.RequestURL)
	}
}

func TestJSONProtocolwriteJSONInputRequestWithQuery(t *testing.T) {
	ci, cancel := setupRequest(nil)
	defer cancel()
	r, w := io.Pipe()
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.writeJSONToContainer(ci)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := &jsonIn{}
	bb := new(bytes.Buffer)

	_, err := bb.ReadFrom(r)
	if err != nil {
		t.Error(err.Error())
	}
	err = json.Unmarshal(bb.Bytes(), incomingReq)
	if err != nil {
		t.Error(err.Error())
	}
	if incomingReq.Protocol.RequestURL != ci.call.URL {
		t.Errorf("Request URL does not match protocol URL: expected: %s, got %s",
			ci.call.URL, incomingReq.Protocol.RequestURL)
	}
}
