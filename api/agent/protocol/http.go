package protocol

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"

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

// this is just an http.Handler really
// TODO handle req.Context better with io.Copy. io.Copy could push us
// over the timeout.
// TODO maybe we should take io.Writer, io.Reader but then we have to
// dump the request to a buffer again :(
func (h *HTTPProtocol) Dispatch(call *models.Call, w io.Writer, req *http.Request) error {
	err := DumpRequestTo(h.in, req) // TODO timeout
	if err != nil {
		return err
	}

	if rw, ok := w.(http.ResponseWriter); ok {
		// if we're writing directly to the response writer, we need to set headers
		// and status code first since calling res.Write will just write the http
		// response as the body (headers and all)

		res, err := http.ReadResponse(bufio.NewReader(h.out), req) // TODO timeout
		if err != nil {
			return err
		}

		for k, vs := range res.Header {
			for _, v := range vs {
				rw.Header().Add(k, v) // on top of any specified on the route
			}
		}
		rw.WriteHeader(res.StatusCode)
		// TODO should we TCP_CORK ?

		io.Copy(rw, res.Body) // TODO timeout
		res.Body.Close()
	} else {
		// logs can just copy the full thing in there, headers and all.

		res, err := http.ReadResponse(bufio.NewReader(h.out), req) // TODO timeout
		if err != nil {
			return err
		}
		res.Write(w) // TODO timeout
	}

	return nil
}

// DumpRequestTo is httputil.DumpRequest with some modifications. It will
// dump the request to the provided io.Writer with the body always, consuming
// the body in the process.
//
// TODO we should support h2!
func DumpRequestTo(w io.Writer, req *http.Request) error {
	// By default, print out the unmodified req.RequestURI, which
	// is always set for incoming server requests. But because we
	// previously used req.URL.RequestURI and the docs weren't
	// always so clear about when to use DumpRequest vs
	// DumpRequestOut, fall back to the old way if the caller
	// provides a non-server Request.

	reqURI := req.RequestURI
	if reqURI == "" {
		reqURI = req.URL.RequestURI()
	}

	fmt.Fprintf(w, "%s %s HTTP/%d.%d\r\n", valueOrDefault(req.Method, "GET"),
		reqURI, req.ProtoMajor, req.ProtoMinor)

	absRequestURI := strings.HasPrefix(req.RequestURI, "http://") || strings.HasPrefix(req.RequestURI, "https://")
	if !absRequestURI {
		host := req.Host
		if host == "" && req.URL != nil {
			host = req.URL.Host
		}

		if host != "" {
			fmt.Fprintf(w, "Host: %s\r\n", host)
		}
	}

	chunked := len(req.TransferEncoding) > 0 && req.TransferEncoding[0] == "chunked"

	if len(req.TransferEncoding) > 0 {
		fmt.Fprintf(w, "Transfer-Encoding: %s\r\n", strings.Join(req.TransferEncoding, ","))
	}

	if req.Close {
		fmt.Fprintf(w, "Connection: close\r\n")
	}

	err := req.Header.WriteSubset(w, reqWriteExcludeHeaderDump)
	if err != nil {
		return err
	}

	io.WriteString(w, "\r\n")

	if req.Body != nil {
		var dest io.Writer = w
		if chunked {
			dest = httputil.NewChunkedWriter(dest)
		}

		// TODO copy w/ ctx
		_, err = io.Copy(dest, req.Body)
		if chunked {
			dest.(io.Closer).Close()
			io.WriteString(w, "\r\n")
		}
	}

	return err
}

var reqWriteExcludeHeaderDump = map[string]bool{
	"Host":              true, // not in Header map anyway
	"Transfer-Encoding": true,
	"Trailer":           true,
}

// Return value if nonempty, def otherwise.
func valueOrDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}
