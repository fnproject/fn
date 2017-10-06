package protocol

import (
	"bytes"
	"testing"
	"net/http"
	"net/url"
	"io/ioutil"
	"io"
	"encoding/json"
)

type RequestData struct {
	A string `json:"a"`
}

func TestJSONProtocolDumpJSONRequestWithData(t *testing.T) {
	req := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "http",
			Host: "localhost:8080",
			Path: "/v1/apps",
			RawQuery: "something=something&etc=etc",
		},
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Host": []string{"localhost:8080"},
			"User-Agent": []string{"curl/7.51.0"},
			"Content-Type": []string{"application/json"},
		},
		Host: "localhost:8080",
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(RequestData{A: "a"})
	req.Body = ioutil.NopCloser(&buf)

	r, w := io.Pipe()
	proto := JSONProtocol{w,r}
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
}

func TestJSONProtocolDumpJSONRequestWithoutData(t *testing.T) {
	req := &http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "http",
			Host: "localhost:8080",
			Path: "/v1/apps",
			RawQuery: "something=something&etc=etc",
		},
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header: http.Header{
			"Host": []string{"localhost:8080"},
			"User-Agent": []string{"curl/7.51.0"},
			"Content-Type": []string{"application/json"},
		},
		Host: "localhost:8080",
	}
	var buf bytes.Buffer
	req.Body = ioutil.NopCloser(&buf)

	r, w := io.Pipe()
	proto := JSONProtocol{w,r}
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
}
