package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"unicode"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/models"
)

var (
	bufPool = &sync.Pool{New: func() interface{} { return new(bytes.Buffer) }}
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
	Body        string          `json:"body"`
	ContentType string          `json:"content_type"`
	Protocol    CallRequestHTTP `json:"protocol"`
}

// jsonOut the expected response from the function container
type jsonOut struct {
	Body        string            `json:"body"`
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
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	body := buf.String()

	in := jsonIn{
		Body:        body,
		ContentType: ci.ContentType,
		CallID:      ci.CallID(),
		Deadline:    ci.Deadline().String(),
		Protocol: CallRequestHTTP{
			Type:       ci.ProtocolType(),
			Method:     ci.Method(),
			RequestURL: ci.RequestURL(),
			Headers:    ci.Headers(),
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
		return models.NewAPIError(http.StatusBadGateway, fmt.Errorf("invalid json response from function err: %v", err))
	}

	_, span = trace.StartSpan(ctx, "dispatch_json_write_response")
	defer span.End()

	rw, ok := w.(http.ResponseWriter)
	if !ok {
		// logs can just copy the full thing in there, headers and all.
		err := json.NewEncoder(w).Encode(jout)
		return checkExcessData(err, decoder)
	}

	// this has to be done for pulling out:
	// - status code
	// - body
	// - headers
	if jout.Protocol != nil {
		p := jout.Protocol
		for k, v := range p.Headers {
			for _, vv := range v {
				rw.Header().Add(k, vv) // on top of any specified on the route
			}
		}
	}
	// after other header setting, top level content_type takes precedence and is
	// absolute (if set). it is expected that if users want to set multiple
	// values they put it in the string, e.g. `"content-type:"application/json; charset=utf-8"`
	// TODO this value should not exist since it's redundant in proto headers?
	if jout.ContentType != "" {
		rw.Header().Set("Content-Type", jout.ContentType)
	}

	// we must set all headers before writing the status, see http.ResponseWriter contract
	if p := jout.Protocol; p != nil && p.StatusCode != 0 {
		rw.WriteHeader(p.StatusCode)
	}

	_, err = io.WriteString(rw, jout.Body)
	return checkExcessData(err, decoder)
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
