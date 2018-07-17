package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"unicode"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/models"
)

// CallRequestHTTP for the protocol that was used by the end user to call this function. We only have HTTP right now.
type CallRequestHTTP struct {
	Type       string      `json:"type"`
	Method     string      `json:"method"`
	RequestURL string      `json:"request_url"`
	Headers    http.Header `json:"headers"`
}

// CallResponseHTTP for the protocol that was used by the end user to call this function. We only have HTTP right now.
type CallResponseHTTP struct {
	StatusCode int         `json:"status_code,omitempty"`
	Headers    http.Header `json:"headers,omitempty"`
}

// jsonIn We're not using this since we're writing JSON directly right now, but trying to keep it current anyways, much easier to read/follow
type jsonIn struct {
	CallID      string          `json:"call_id"`
	Deadline    string          `json:"deadline"`
	Body        json.RawMessage `json:"body"`
	ContentType string          `json:"content_type"`
	Protocol    CallRequestHTTP `json:"protocol"`
}

// jsonOut the expected response from the function container
type jsonOut struct {
	Body        json.RawMessage   `json:"body"`
	ContentType string            `json:"content_type"`
	Protocol    *CallResponseHTTP `json:"protocol,omitempty"`
}

// JSONProtocol converts stdin/stdout streams from HTTP into JSON format.
type JSONProtocol struct {
	// These are the container input streams, not the input from the request or the output for the response
	in  io.Writer
	out io.Reader
}

func (p *JSONProtocol) IsStreamable() bool {
	return true
}

func (h *JSONProtocol) writeJSONToContainer(ci *event.Event) error {

	callID, err := ci.GetCallID()
	if err != nil {
		return err
	}

	deadline, err := ci.GetDeadline()
	if err != nil {
		return err
	}

	var method, requestURL string
	var headers http.Header

	if ci.HasExtension(event.ExtIoFnProjectHTTPReq) {
		var ext event.HTTPReqExt
		err = ci.ReadExtension(event.ExtIoFnProjectHTTPReq, &ext)
		if err != nil {
			fmt.Errorf("invalid HTTP metadata on incoming event: %s", err)
		}
		method = ext.Method
		requestURL = ext.RequestURL
		headers = ext.Headers
	} else {
		method = "GET"
		requestURL = "http://example.com"
	}

	in := jsonIn{
		Body:        ci.Data,
		ContentType: ci.ContentType,
		CallID:      callID,
		Deadline:    deadline.String(),
		Protocol: CallRequestHTTP{
			Type:       "http",
			Method:     method,
			RequestURL: requestURL,
			Headers:    headers,
		},
	}

	return json.NewEncoder(h.in).Encode(in)
}

func (h *JSONProtocol) Dispatch(ctx context.Context, ci *event.Event) (*event.Event, error) {
	ctx, span := trace.StartSpan(ctx, "dispatch_json")
	defer span.End()

	_, span = trace.StartSpan(ctx, "dispatch_json_write_request")
	err := h.writeJSONToContainer(ci)
	span.End()
	if err != nil {
		return nil, err
	}

	_, span = trace.StartSpan(ctx, "dispatch_json_read_response")
	var jout jsonOut
	decoder := json.NewDecoder(h.out)
	err = decoder.Decode(&jout)
	span.End()
	if err != nil {
		return nil, models.NewAPIError(http.StatusBadGateway, fmt.Errorf("invalid json response from function err: %v", err))
	}

	_, span = trace.StartSpan(ctx, "dispatch_json_write_response")
	defer span.End()

	var headers http.Header
	status := 200
	// this has to be done for pulling out:
	// - status code
	// - body
	// - headers
	if jout.Protocol != nil {
		status = jout.Protocol.StatusCode
		p := jout.Protocol

		for k, v := range p.Headers {
			for _, vv := range v {
				// largely do this to normalise header names
				headers.Add(k, vv)
			}
		}
	}

	evt, err := event.CreateHttpRespEvent("http://fnproject.io", jout.Body, jout.ContentType, status, headers)
	if err != nil {
		return nil, err
	}
	return evt, checkExcessData(decoder)
}

func checkExcessData(decoder *json.Decoder) error {
	// Now check for excess output, if this is the case, we can be certain that the next request will fail.
	reader, ok := decoder.Buffered().(*bytes.Reader)
	if ok && reader.Len() > 0 {
		// Let's check if extra data is whitespace, which is valid/ignored in json
		for {
			r, _, err := reader.ReadRune()
			if err == io.EOF {
				break
			}
			if !unicode.IsSpace(r) {
				return ErrExcessData
			}
		}
	}

	return nil
}
