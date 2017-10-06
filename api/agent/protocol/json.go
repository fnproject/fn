package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// This is sent into the function
// All HTTP request headers should be set in env
type JSONIO struct {
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

type RequestEncoder struct {
	*json.Encoder
}

func (h *JSONProtocol) DumpJSON(w io.Writer, req *http.Request) error {
	stdin := json.NewEncoder(h.in)
	_, err := io.WriteString(h.in, `{`)
	if err != nil {
		// this shouldn't happen
		return err
	}

	if req.ContentLength != 0 {
		_, err := io.WriteString(h.in, `"body": `)
		if err != nil {
			// this shouldn't happen
			return err
		}
		bb := new(bytes.Buffer)
		_, err = bb.ReadFrom(req.Body)
		if err != nil {
			return err
		}

		err = stdin.Encode(bb.String())
		if err != nil {
			return err
		}
		_, err = io.WriteString(h.in, `,`)
		if err != nil {
			// this shouldn't happen
			return err
		}
		defer bb.Reset()
	}
	_, err = io.WriteString(h.in, `"headers:"`)
	if err != nil {
		// this shouldn't happen
		return err
	}
	err = stdin.Encode(req.Header)
	if err != nil {
		// this shouldn't happen
		return err
	}
	_, err = io.WriteString(h.in, `"}`)
	if err != nil {
		// this shouldn't happen
		return err
	}
	return nil
}

func (h *JSONProtocol) Dispatch(w io.Writer, req *http.Request) error {
	err := h.DumpJSON(w, req)
	if err != nil {
		return err
	}
	jout := new(JSONIO)
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
