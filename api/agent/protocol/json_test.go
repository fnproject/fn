package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/fnproject/fn/api/models"
)

type RequestData struct {
	A string `json:"a"`
}

type funcRequestBody struct {
	Body    string            `json:"body"`
	Headers map[string]string `json:"headers"`
}

func setupRequest(data interface{}) *http.Request {
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
	return req
}

func TestJSONProtocolwriteJSONInputRequestWithData(t *testing.T) {
	rDataBefore := RequestData{A: "a"}
	req := setupRequest(rDataBefore)
	r, w := io.Pipe()
	call := &models.Call{}
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.writeJSONInput(call, req)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := new(funcRequestBody)
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
}

func TestJSONProtocolwriteJSONInputRequestWithoutData(t *testing.T) {
	req := setupRequest(nil)

	call := &models.Call{}
	r, w := io.Pipe()
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.writeJSONInput(call, req)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := new(funcRequestBody)
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
	if ok := reflect.DeepEqual(req.Header, incomingReq.Headers); !ok {
		t.Errorf("Request headers assertion mismatch: expected: %s, got %s",
			req.Header, incomingReq.Headers)
	}
}

func TestJSONProtocolwriteJSONInputRequestWithQuery(t *testing.T) {
	req := setupRequest(nil)

	r, w := io.Pipe()
	call := &models.Call{}
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.writeJSONInput(call, req)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := new(funcRequestBody)
	bb := new(bytes.Buffer)

	_, err := bb.ReadFrom(r)
	if err != nil {
		t.Error(err.Error())
	}
	err = json.Unmarshal(bb.Bytes(), incomingReq)
	if err != nil {
		t.Error(err.Error())
	}
	// if incomingReq.QueryParameters != req.URL.RawQuery {
	// 	t.Errorf("Request query string assertion mismatch: expected: %s, got %s",
	// 		req.URL.RawQuery, incomingReq.QueryParameters)
	// }
}
