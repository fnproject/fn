package protocol

import (
	"bufio"
	"context"
	"io"
	"net/http"
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
	req := ci.Request()

	req.RequestURI = ci.RequestURL() // force set to this, for req.Write to use (TODO? still?)

	// Add Fn-specific headers for this protocol
	req.Header.Set("FN_DEADLINE", ci.Deadline().String())
	req.Header.Set("FN_METHOD", ci.Method())
	req.Header.Set("FN_REQUEST_URL", ci.RequestURL())
	req.Header.Set("FN_CALL_ID", ci.CallID())

	// req.Write handles if the user does not specify content length
	err := req.Write(h.in)
	if err != nil {
		return err
	}

	resp, err := http.ReadResponse(bufio.NewReader(h.out), ci.Request())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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
