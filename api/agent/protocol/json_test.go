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
)

type RequestData struct {
	A string `json:"a"`
}

type fuckReed struct {
	Body RequestData `json:"body"`
}

func TestJSONProtocolDumpJSONRequestWithData(t *testing.T) {
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
	rDataBefore := RequestData{A: "a"}
	json.NewEncoder(&buf).Encode(rDataBefore)
	req.Body = ioutil.NopCloser(&buf)

	r, w := io.Pipe()
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.DumpJSON(req)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := new(jsonio)
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

func TestJSONProtocolDumpJSONRequestWithoutData(t *testing.T) {
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
	req.Body = ioutil.NopCloser(&buf)

	r, w := io.Pipe()
	proto := JSONProtocol{w, r}
	go func() {
		err := proto.DumpJSON(req)
		if err != nil {
			t.Error(err.Error())
		}
		w.Close()
	}()
	incomingReq := new(jsonio)
	bb := new(bytes.Buffer)

	_, err := bb.ReadFrom(r)
	if err != nil {
		t.Error(err.Error())
	}
	err = json.Unmarshal(bb.Bytes(), incomingReq)
	if err != nil {
		t.Error(err.Error())
	}
	if ok := reflect.DeepEqual(req.Header, incomingReq.Headers); !ok {
		t.Errorf("Request headers assertion mismatch: expected: %s, got %s",
			req.Header, incomingReq.Headers)

	}
}
