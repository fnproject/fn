package protocol

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/models"
)

// HTTPProtocol converts stdin/stdout streams into HTTP/1.1 compliant
// communication. It relies on Content-Length to know when to stop reading from
// containers stdout. It also mandates valid HTTP headers back and forth, thus
// returning errors in case of parsing problems.
type HTTPProtocol struct {
	in  io.Writer
	out io.Reader
}

func (p *HTTPProtocol) IsStreamable() bool { return true }

func (h *HTTPProtocol) Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error {
	ctx, span := trace.StartSpan(ctx, "dispatch_http")
	defer span.End()

	req, err := http.NewRequest(ci.Method(), ci.RequestURL(), ci.Input())
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	req.Header = http.Header(ci.Headers())

	req.RequestURI = ci.RequestURL() // force set to this, for req.Write to use (TODO? still?)

	// Add Fn-specific headers for this protocol
	req.Header.Set("FN_DEADLINE", ci.Deadline().String())
	req.Header.Set("FN_METHOD", ci.Method())
	req.Header.Set("FN_REQUEST_URL", ci.RequestURL())
	req.Header.Set("FN_CALL_ID", ci.CallID())

	_, span = trace.StartSpan(ctx, "dispatch_http_write_request")
	// req.Write handles if the user does not specify content length
	err = req.Write(h.in)
	span.End()
	if err != nil {
		return err
	}

	_, span = trace.StartSpan(ctx, "dispatch_http_read_response")
	resp, err := http.ReadResponse(bufio.NewReader(h.out), req)
	span.End()
	if err != nil {
		return models.NewAPIError(http.StatusBadGateway, fmt.Errorf("invalid http response from function err: %v", err))
	}

	_, span = trace.StartSpan(ctx, "dispatch_http_write_response")
	defer span.End()

	rw, ok := w.(http.ResponseWriter)
	if !ok {
		// async / [some] tests go through here. write a full http request to the writer
		resp.Write(w)
		return nil
	}

	// if we're writing directly to the response writer, we need to set headers
	// and status code, and only copy the body. resp.Write would copy a full
	// http request into the response body (not what we want).

	// add resp's on top of any specified on the route [on rw]
	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}
	if resp.StatusCode > 0 {
		rw.WriteHeader(resp.StatusCode)
	}
	io.Copy(rw, resp.Body)
	return nil
}
