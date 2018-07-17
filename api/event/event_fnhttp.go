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

	DefaultCloudEventVersion = "0.1"

	fallbackContentType = "application/octet-stream"
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
	Headers map[string][]string `json:"headers,omitempty"`
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

// This does JSON convertion - taking the body of an HTTP message and returning it as a JSON body to insert into a cloud event message
func convertHTTPBodyToJsonBody(contentType string, body []byte, maxBodySize uint64) (json.RawMessage, string, error) {

	if contentType == "" {
		// TODO - not super good
		contentType = fallbackContentType
	}

	mediaType, _, err := mime.ParseMediaType(contentType)

	if err != nil {
		return nil, "", errors.New("invalid content-type header")
	}

	if mediaType == "application/json" { // be smarter about what this is
		if !json.Valid(body) {
			return nil, "", ErrInvalidJSONBody
		}
		return json.RawMessage(body), contentType, nil
	} else {
		// TODO Dubious about this maybe just skip in favour of JSON only input
		r, s := utf8.DecodeLastRune(body)
		if s == 1 && r == utf8.RuneError {
			return nil, "", ErrUnsupportedBodyEncoding
		}

		newBuf := &bytes.Buffer{}
		bodyW := common.NewClampWriter(newBuf, maxBodySize, ErrEncodedBodyTooLong)

		err := json.NewEncoder(bodyW).Encode(string(body))

		if err != nil {
			return nil, "", err
		}
		return json.RawMessage(newBuf.Bytes()), contentType, nil
	}

}

// FromHTTPTriggerRequest creates an FN HTTP Request cloud event from an HTTP req
// This will buffer the whole request into RAM
func FromHTTPTriggerRequest(r *http.Request, maxBodySize uint64) (*Event, error) {

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
		rawData, contentType, err = convertHTTPBodyToJsonBody(r.Header.Get("Content-type"), body, maxBodySize)
		if err != nil {
			return nil, err
		}
	}

	rUrl := toFullURL(r)
	reqExt := HTTPReqExt{
		Method:     r.Method,
		Headers:    r.Header,
		RequestURL: rUrl,
	}

	evt := &Event{
		CloudEventsVersion: DefaultCloudEventVersion,
		EventType:          EventTypeHTTPReq,
		Data:               rawData,
		EventTypeVersion:   EventTypeHTTPRespVersion,
		EventTime:          common.DateTime(time.Now()),
		Source:             rUrl,
		ContentType:        contentType,
		EventID:            id.New().String(),
	}

	err = evt.SetExtension(ExtIoFnProjectHTTPReq, reqExt)
	if err != nil {
		return nil, err
	}

	return evt, nil
}

func CreateHttpRespEvent(sourceID string, body json.RawMessage, contentType string, status int, headers map[string][]string) (*Event, error) {
	respExt := HTTPRespExt{
		Status:  status,
		Headers: headers,
	}

	evt := &Event{
		CloudEventsVersion: DefaultCloudEventVersion,
		EventType:          EventTypeHTTPResp,
		Data:               body,
		ContentType:        contentType,
		EventTime:          common.DateTime(time.Now()),
		EventTypeVersion:   EventTypeHTTPReqVersion,
		EventID:            id.New().String(),
		Source:             sourceID,
	}

	err := evt.SetExtension(ExtIoFnProjectHTTPResp, respExt)
	if err != nil {
		return nil, err
	}
	return evt, nil

}

//FromHTTPResponse Creates an Fn http response event from a given HTTP response - this can be used to (e.g. parse the response of an HTTP container and turn it into a cloud event
func FromHTTPResponse(sourceID string, maxBodySize uint64, r *http.Response) (*Event, error) {

	buf := bytes.Buffer{}
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		return nil, err
	}

	body := buf.Bytes()
	var rawData json.RawMessage
	var contentType string

	if len(body) > 0 {
		rawData, contentType, err = convertHTTPBodyToJsonBody(r.Header.Get("Content-type"), body, maxBodySize)
		if err != nil {
			return nil, err
		}
	}

	return CreateHttpRespEvent(sourceID, rawData, contentType, r.StatusCode, r.Header)

}
