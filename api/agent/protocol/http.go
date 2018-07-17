package protocol

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"

	"go.opencensus.io/trace"

	"bytes"
	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/models"
	"github.com/pkg/errors"
)

// httpProtocol converts stdin/stdout streams into HTTP/1.1 compliant
// communication. It relies on Content-Length to know when to stop reading from
// containers stdout. It also mandates valid HTTP headers back and forth, thus
// returning errors in case of parsing problems.
type httpProtocol struct {
	in  io.Writer
	out io.Reader
}

func (p *httpProtocol) IsStreamable() bool { return true }

func (h *httpProtocol) Dispatch(ctx context.Context, evt *event.Event) (*event.Event, error) {
	ctx, span := trace.StartSpan(ctx, "dispatch_http")
	defer span.End()

	var method, requestURL string
	var headers http.Header

	if evt.HasExtension(event.ExtIoFnProjectHTTPReq) {
		var httpState event.HTTPReqExt
		err := evt.ReadExtension(event.ExtIoFnProjectHTTPReq, &httpState)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Invalid %s extension data, %s", event.ExtIoFnProjectHTTPReq, err.Error()))
		}

		method = httpState.Method
		headers = make(http.Header)
		for k, vs := range httpState.Headers {
			for _, v := range vs {
				headers.Add(k, v)
			}
		}
		requestURL = httpState.RequestURL
	} else {
		method = http.MethodGet
		requestURL = "http://fnproject.io/non-http-input"
	}

	req, err := http.NewRequest(method, requestURL, bytes.NewReader(evt.Data))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header = http.Header(headers)

	req.RequestURI = requestURL // force set to this, for req.Write to use (TODO? still?)

	// TODO these should be "must never happens" really - consider wrapping input event again
	deadline, err := evt.GetDeadline()
	if err != nil {
		return nil, fmt.Errorf("invalid %s extension data, %s", event.ExtIoFnProjectDeadline, err)
	}
	callID, err := evt.GetCallID()
	if err != nil {
		return nil, fmt.Errorf("invalid %s extension data, %s", event.ExtIoFnProjectCallID, err)
	}

	// Add Fn-specific headers for this protocol
	req.Header.Set("FN_DEADLINE", deadline.String())
	req.Header.Set("FN_METHOD", method)
	req.Header.Set("FN_REQUEST_URL", requestURL)
	req.Header.Set("FN_CALL_ID", callID)

	_, span = trace.StartSpan(ctx, "dispatch_http_write_request")
	// req.Write handles if the user does not specify content length
	err = req.Write(h.in)
	span.End()
	if err != nil {
		return nil, models.NewAPIError(http.StatusBadGateway, fmt.Errorf("Error writing message to container : %s ", err))
	}

	_, span = trace.StartSpan(ctx, "dispatch_http_read_response")
	resp, err := http.ReadResponse(bufio.NewReader(h.out), req)
	span.End()
	if err != nil {
		return nil, models.NewAPIError(http.StatusBadGateway, fmt.Errorf("invalid http response from function err: %v", err))
	}

	_, span = trace.StartSpan(ctx, "dispatch_http_write_response")
	defer span.End()

	// if we're writing directly to the response writer, we need to set headers
	// and status code, and only copy the body. resp.Write would copy a full
	// http request into the response body (not what we want).

	// TODO: INVOKETODO : max resp size config, source ID
	respEvent, err := event.FromHTTPResponse("http://fnproject.io", 1024*1024, resp)

	if err != nil {
		return nil, models.NewAPIError(http.StatusBadGateway,
			fmt.Errorf("failed to read http response from container %s", err))
	}
	return respEvent, nil
}
