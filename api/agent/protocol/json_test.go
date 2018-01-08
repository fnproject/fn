package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/fnproject/fn/api/models"
)

type RequestData struct {
	A string `json:"a"`
}

func setupRequest(data interface{}) *callInfoImpl {
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

	call := &models.Call{Type: "json"}

	// fixup URL in models.Call
	call.URL = req.URL.String()

	ci := &callInfoImpl{call, req}
	return ci
}

func TestJSONProtocolwriteJSONInputRequestWithData(t *testing.T) {
	rDataBefore := RequestData{A: "a"}
	ci := setupRequest(rDataBefore)
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
	if incomingReq.Protocol.Type != ci.call.Type {
		t.Errorf("Call protocol type assertion mismatch: expected: %s, got %s",
			ci.call.Type, incomingReq.Protocol.Type)
	}
}

func TestJSONProtocolwriteJSONInputRequestWithoutData(t *testing.T) {
	ci := setupRequest(nil)
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
	if !models.Headers(ci.req.Header).Equals(models.Headers(incomingReq.Protocol.Headers)) {
		t.Errorf("Request headers assertion mismatch: expected: %s, got %s",
			ci.req.Header, incomingReq.Protocol.Headers)
	}
}

func TestJSONProtocolwriteJSONInputRequestWithQuery(t *testing.T) {
	ci := setupRequest(nil)
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
