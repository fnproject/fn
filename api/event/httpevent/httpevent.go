package httpevent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/id"
	"mime"
	"net/http"
	"strconv"
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

// FromHTTPRequest creates an FN HTTP Request cloud event from an HTTP req
// This will buffer the whole request into RAM
func FromHTTPRequest(r *http.Request, maxBodySize uint64) (*event.Event, error) {

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

	evt := &event.Event{
		CloudEventsVersion: DefaultCloudEventVersion,
		EventType:          EventTypeHTTPReq,
		Data:               rawData,
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

func CreateHttpRespEvent(sourceID string, body json.RawMessage, contentType string, status int, headers map[string][]string) (*event.Event, error) {
	respExt := HTTPRespExt{
		Status:  status,
		Headers: headers,
	}

	evt := &event.Event{
		CloudEventsVersion: DefaultCloudEventVersion,
		EventType:          EventTypeHTTPResp,
		Data:               body,
		ContentType:        contentType,
		EventTime:          common.DateTime(time.Now()),
		EventID:            id.New().String(),
		Source:             sourceID,
	}

	err := evt.SetExtension(ExtIoFnProjectHTTPResp, respExt)
	if err != nil {
		return nil, err
	}
	return evt, nil

}

// TODO test
// WriteHTTPResponse emits an event as a raw HTTP response , outputting the body of the response if any in the HTTP body and honoruing any HTTP extensions in the event
func WriteHTTPResponse(ctx context.Context, event *event.Event, resp http.ResponseWriter) error {

	var respMeta HTTPRespExt

	if event.HasExtension(ExtIoFnProjectHTTPResp) {
		err := event.ReadExtension(ExtIoFnProjectHTTPResp, &respMeta)
		if err != nil {
			return err
		}
	} else {
		respMeta = HTTPRespExt{
			Status: 200,
		}
	}

	for k, vs := range respMeta.Headers {
		for _, v := range vs {
			resp.Header().Add(k, v)
		}
	}

	if event.ContentType != "" {
		resp.Header().Set("Content-type", event.ContentType)
	}
	var body []byte
	if event.Data != nil {
		bodyString, err := event.BodyAsRawString()
		if err != nil {
			return err
		}
		body = []byte(bodyString)
		resp.Header().Set("Content-Length", strconv.Itoa(len(body)))
	}

	resp.WriteHeader(respMeta.Status)

	if body != nil {
		_, err := resp.Write(body)
		if err != nil {
			return err
		}
	}
	return nil
}

//FromHTTPResponse Creates an Fn http response event from a given HTTP response - this can be used to (e.g. parse the response of an HTTP container and turn it into a cloud event
func FromHTTPResponse(ctx context.Context, sourceID string, maxBodySize uint64, r *http.Response) (*event.Event, error) {

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
