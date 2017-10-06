package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// This is sent into the function
// All HTTP request headers should be set in env
type jsonio struct {
	Headers    http.Header `json:"headers,omitempty"`
	Body       string      `json:"body"`
	StatusCode int         `json:"status_code,omitempty"`
}

// JSONProtocol converts stdin/stdout streams from HTTP into JSON format.
type JSONProtocol struct {
	in  io.Writer
	out io.Reader
}

func (p *JSONProtocol) IsStreamable() bool {
	return true
}

func writeString(err error, dst io.Writer, str string) error {
	if err != nil {
		return err
	}
	_, err = io.WriteString(dst, str)
	return err
}

func (h *JSONProtocol) DumpJSON(req *http.Request) error {
	stdin := json.NewEncoder(h.in)
	bb := new(bytes.Buffer)
	_, err := bb.ReadFrom(req.Body)
	if err != nil {
		return err
	}
	err = writeString(err, h.in, "{")
	err = writeString(err, h.in, `"body":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(bb.String())
	err = writeString(err, h.in, ",")
	defer bb.Reset()
	err = writeString(err, h.in, `"headers":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(req.Header)
	err = writeString(err, h.in, "}")
	return err
}

func (h *JSONProtocol) Dispatch(w io.Writer, req *http.Request) error {
	err := h.DumpJSON(req)
	if err != nil {
		return err
	}
	jout := new(jsonio)
	dec := json.NewDecoder(h.out)
	if err := dec.Decode(jout); err != nil {
		return err
	}
	if rw, ok := w.(http.ResponseWriter); ok {
		// this has to be done for pulling out:
		// - status code
		// - body
		// - headers
		for k, vs := range jout.Headers {
			for _, v := range vs {
				rw.Header().Add(k, v) // on top of any specified on the route
			}
		}
		rw.WriteHeader(jout.StatusCode)
		_, err = io.WriteString(rw, jout.Body) // TODO timeout
		if err != nil {
			return err
		}
	} else {
		// logs can just copy the full thing in there, headers and all.
		err = json.NewEncoder(w).Encode(jout)
		if err != nil {
			return err
		}
	}
	return nil
}
