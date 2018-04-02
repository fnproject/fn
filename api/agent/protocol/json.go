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

type errorOut struct {
	Message string `json:"message"`
	Type    string `json:"type,omitempty"`
	Trace   string `json:"trace,omitempty"`
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
	Error       *errorOut         `json:"error,omitempty"`
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

func (h *JSONProtocol) writeJSONToContainer(ci CallInfo) error {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	_, err := io.Copy(buf, ci.Input())
	if err != nil {
		return err
	}

	body := buf.String()

	in := jsonIn{
		Body:        body,
		ContentType: ci.ContentType(),
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

func (h *JSONProtocol) Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error {
	fmt.Println("JSON Dispatch")

	ctx, span := trace.StartSpan(ctx, "dispatch_json")
	defer span.End()

	_, span = trace.StartSpan(ctx, "dispatch_json_write_request")
	err := h.writeJSONToContainer(ci)
	span.End()
	if err != nil {
		return err
	}
	fmt.Println("FUCK 2")
	_, span = trace.StartSpan(ctx, "dispatch_json_read_response")
	var jout jsonOut
	// buf := new(bytes.Buffer)
	// buf.ReadFrom(h.out)
	// s := buf.String()
	// log.Println("output string:", s)
	decoder := json.NewDecoder(h.out)
	err = decoder.Decode(&jout)
	span.End()
	if err != nil {
		fmt.Println("FUCKER", err)
		return models.NewAPIError(http.StatusBadGateway, fmt.Errorf("invalid json response from function err: %v", err))
	}
	fmt.Println("FUCK 3")
	_, span = trace.StartSpan(ctx, "dispatch_json_write_response")
	defer span.End()

	rw, ok := w.(http.ResponseWriter)
	if !ok {
		// logs can just copy the full thing in there, headers and all.
		err := json.NewEncoder(w).Encode(jout)
		return h.isExcessData(err, decoder)
	}
	fmt.Println("FUCK 4")
	body := jout.Body
	statusCode := 200
	fmt.Printf("JOUT: %+v\n", jout)
	if jout.Error != nil {
		fmt.Println("ERROR RESPONSE")
		// then we'll do an error response, protocol specific things override these
		statusCode = 500
		rw.Header().Set("Content-Type", "application/json")
		b, err := json.Marshal(jout.Error)
		if err != nil {
			// this should never happen...
			return models.NewAPIError(http.StatusInternalServerError, fmt.Errorf("error marshalling error output. This shouldn't happen, please file an issue at github.com/fnproject/fn: %v", err))
		}
		body = string(b)
	}

	// absolute (if set). it is expected that if users want to set multiple
	// values they put it in the string, e.g. `"content-type:"application/json; charset=utf-8"`
	if jout.ContentType != "" {
		rw.Header().Set("Content-Type", jout.ContentType)
	}

	// Protocol specific values take precedence over top level values.
	// this has to be done for pulling out:
	// - status code
	// - body
	// - headers
	p := jout.Protocol
	if p != nil {
		if p.StatusCode != 0 {
			statusCode = p.StatusCode
		}
		for k, v := range p.Headers {
			for _, vv := range v {
				// Set() so it overrides anything else
				rw.Header().Set(k, vv) // on top of any specified on the route
			}
		}
	}

	fmt.Printf("writing headers: %+v\n", rw.Header())
	// we must set all headers before writing the status, see http.ResponseWriter contract
	rw.WriteHeader(statusCode)

	_, err = io.WriteString(rw, body)
	return h.isExcessData(err, decoder)
}

func (h *JSONProtocol) isExcessData(err error, decoder *json.Decoder) error {
	if err == nil {
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
	}
	return err
}
