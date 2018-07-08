package cloudevent

import (
	"net/http"
	"github.com/fnproject/fn/api/agent/protocol"
	"encoding/json"
	"mime"
	"github.com/pkg/errors"
	"bytes"
	"text/scanner"
)

const (
	EventTypeHTTPReq  = "io.fnproject.httpRequest"
	EventTypeHTTPResp = "io.fnproject.httpResponse"

	ExtIoFnProjectHTTPReq  = "ioFnProjectHTTPReq"
	ExtIoFnProjectHTTPResp = "ioFnProjectHTTPResp"

	cloudEventsVersion = "0.2"
)

type HTTPReqExt struct {
	Method     string              `json:"method"`
	Headers    map[string][]string `json:"headers"`
	RequestURL string              `json:"requestURL"`
}

type HTTPRespExt struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
}

type rawJSONBody byte[]

// FromHTTPReq creates an FN HTTP Request cloud event from an HTTP req
// This will buffer the whole request into RAM
func FromHTTPReq(r http.Request) (*protocol.CloudEvent, error) {

	contentType := r.Header.Get("Content-type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	mediaType, _, err := mime.ParseMediaType(contentType)

	if err != nil {
		return errors.New("Invalid content-type header")
	}

	var bytesBody []byte
	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		return err, nil
	}

	s := scanner.Scanner{}
	s.String()
	if mediaType == "application/json" { // be smarter about what this is
		body := rawJsonBody(buf.Bytes())
		if !json.Valid(body) {
			return errors.New("Invalid JSON body")
		}
	}else if  {

	}

	json.Valid()
	body :=
	evt := &protocol.CloudEvent{
		CloudEventsVersion: cloudEventsVersion,
		EventType:          EventTypeHTTPReq,
		Data: body,
	}

}
