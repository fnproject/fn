package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fnproject/fn/api/models"
)

// This is sent into the function
// All HTTP request headers should be set in env
type jsonio struct {
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body"`
}

type jsonIn struct {
	jsonio
	Config map[string]string `json:"config,omitempty"`
}
type jsonOut struct {
	jsonio
	StatusCode int `json:"status_code,omitempty"`
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

// TODO(xxx): headers, query parameters, body - what else should we add to func's payload?
// TODO(xxx): get rid of request body buffering somehow
func (h *JSONProtocol) writeJSONInput(call *models.Call, req *http.Request) error {
	stdin := json.NewEncoder(h.in)
	bb := new(bytes.Buffer)
	_, err := bb.ReadFrom(req.Body)
	// todo: better/simpler err handling
	if err != nil {
		return err
	}
	// open
	err = writeString(err, h.in, "{")
	if err != nil {
		return err
	}

	// call_id
	err = writeString(err, h.in, `"call_id":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(call.ID)
	if err != nil {
		return err
	}

	// body
	err = writeString(err, h.in, ",")
	err = writeString(err, h.in, `"body":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(bb.String())
	if err != nil {
		return err
	}

	// request URL
	err = writeString(err, h.in, ",")
	err = writeString(err, h.in, `"request_url":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(req.URL.String())
	if err != nil {
		return err
	}

	// headers
	err = writeString(err, h.in, ",")
	err = writeString(err, h.in, `"headers":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(req.Header)

	// config
	if call.EnvVars != nil && len(call.EnvVars) > 0 {
		err = writeString(err, h.in, ",")
		err = writeString(err, h.in, `"config":`)
		if err != nil {
			return err
		}
		err = stdin.Encode(call.EnvVars)
	}

	// close
	err = writeString(err, h.in, "}\n\n")
	return err
}

func (h *JSONProtocol) Dispatch(call *models.Call, w io.Writer, req *http.Request) error {
	err := h.writeJSONInput(call, req)
	if err != nil {
		return err
	}
	jout := new(jsonOut)
	dec := json.NewDecoder(h.out)
	if err := dec.Decode(jout); err != nil {
		return fmt.Errorf("error decoding JSON from user function: %v", err)
	}
	if rw, ok := w.(http.ResponseWriter); ok {
		// this has to be done for pulling out:
		// - status code
		// - body
		// - headers
		for k, v := range jout.Headers {
			rw.Header().Set(k, v) // on top of any specified on the route
		}
		if jout.StatusCode != 0 {
			rw.WriteHeader(jout.StatusCode)
		} else {
			rw.WriteHeader(http.StatusOK)
		}
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
