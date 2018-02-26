package protocol

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/fnproject/fn/api/models"
	opentracing "github.com/opentracing/opentracing-go"
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
	span, ctx := opentracing.StartSpanFromContext(ctx, "dispatch_http")
	defer span.Finish()

	req := ci.Request()

	req.RequestURI = ci.RequestURL() // force set to this, for req.Write to use (TODO? still?)

	// Add Fn-specific headers for this protocol
	req.Header.Set("FN_DEADLINE", ci.Deadline().String())
	req.Header.Set("FN_METHOD", ci.Method())
	req.Header.Set("FN_REQUEST_URL", ci.RequestURL())
	req.Header.Set("FN_CALL_ID", ci.CallID())

	span, _ = opentracing.StartSpanFromContext(ctx, "dispatch_http_write_request")
	// req.Write handles if the user does not specify content length
	err := req.Write(h.in)
	span.Finish()
	if err != nil {
		return err
	}

	span, _ = opentracing.StartSpanFromContext(ctx, "dispatch_http_read_response")
	resp, err := http.ReadResponse(bufio.NewReader(h.out), ci.Request())
	span.Finish()
	if err != nil {
		return models.NewAPIError(http.StatusBadGateway, fmt.Errorf("invalid http response from function err: %v", err))
	}

	span, _ = opentracing.StartSpanFromContext(ctx, "dispatch_http_write_response")
	defer span.Finish()

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
