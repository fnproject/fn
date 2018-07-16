package event

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"mime"
	"net/http"
	"time"
	"unicode/utf8"
)

const (
	EventTypeHTTPReq         = "io.fnproject.httpRequest"
	EventTypeHTTPReqVersion  = "0.1"
	EventTypeHTTPResp        = "io.fnproject.httpResponse"
	EventTypeHTTPRespVersion = "0.1"

	ExtIoFnProjectHTTPReq  = "ioFnProjectHTTPReq"
	ExtIoFnProjectHTTPResp = "ioFnProjectHTTPResp"

	cloudEventsVersion = "0.1"

	// TODO cap this properly in config
	maxBodySize = 1024 * 1024 * 1024
)

// TODO copypasta from api/error.go
type err struct {
	code int
	error
}

func (e err) Code() int { return e.code }

var (
	ErrUnsupportedBodyEncoding = err{code: 400, error: errors.New("unsupported body encoding, only strings and valid JSON documents are supported")}
	ErrEncodedBodyTooLong      = err{code: 400, error: errors.New("unsupported body encoded body exceeds max size")}
	ErrInvalidJSONBody         = err{code: 400, error: errors.New("invalid JSON document")}
)

type HTTPReqExt struct {
	Method     string              `json:"method"`
	Headers    map[string][]string `json:"headers,omitempty"`
	RequestURL string              `json:"requestURL,omitempty"`
}

type HTTPRespExt struct {
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers"`
}

func toFullURL(r *http.Request) string {

	if r.URL.IsAbs() {
		return r.URL.String()
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host

	return fmt.Sprintf("%s://%s%s", scheme, host, r.URL.String())

}

// FromHTTPTrigger creates an FN HTTP Request cloud event from an HTTP req
// This will buffer the whole request into RAM
func FromHTTPTrigger(r *http.Request) (*Event, error) {

	// TODO - this is a bit heap-happy
	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		return nil, err
	}

	body := buf.Bytes()

	contentType := ""
	var rawData json.RawMessage
	if len(body) > 0 {
		contentType := r.Header.Get("Content-Type")
		if contentType == "" {
			// TODO - not super good
			contentType = "application/octet-stream"
		}

		mediaType, _, err := mime.ParseMediaType(contentType)

		if err != nil {
			return nil, errors.New("invalid content-type header")
		}
		if mediaType == "application/json" { // be smarter about what this is
			if !json.Valid(body) {
				return nil, ErrInvalidJSONBody
			}
			rawData = json.RawMessage(body)
		} else {
			// TODO Dubious about this maybe just skip in favour of JSON only input
			r, s := utf8.DecodeLastRune(body)
			if s == 1 && r == utf8.RuneError {
				return nil, ErrUnsupportedBodyEncoding
			}

			newBuf := &bytes.Buffer{}
			bodyW := common.NewClampWriter(newBuf, maxBodySize, ErrEncodedBodyTooLong)

			err := json.NewEncoder(bodyW).Encode(string(body))

			if err != nil {
				return nil, err
			}
			rawData = json.RawMessage(newBuf.Bytes())
		}
	}

	rUrl := toFullURL(r)
	reqExt := HTTPReqExt{
		Method:     r.Method,
		Headers:    r.Header,
		RequestURL: rUrl,
	}

	rextSer, err := json.Marshal(reqExt)
	if err != nil {
		return nil, err
	}
	evt := &Event{
		CloudEventsVersion: cloudEventsVersion,
		EventType:          EventTypeHTTPReq,
		Data:               rawData,
		EventTypeVersion:   EventTypeHTTPReqVersion,
		EventTime:          common.DateTime(time.Now()),
		Source:             rUrl,
		ContentType:        contentType,
		EventID:            id.New().String(),
		Extensions: map[string]json.RawMessage{
			ExtIoFnProjectHTTPReq: json.RawMessage(rextSer),
		},
	}

	return evt, nil

}
