package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// This is sent into the function
// All HTTP request headers should be set in env
type jsonio struct {
	Body        string `json:"body"`
	ContentType string `json:"content_type"`
}

// CallRequestHTTP for the protocol that was used by the end user to call this function. We only have HTTP right now.
type CallRequestHTTP struct {
	// TODO request method ?
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
	jsonio
	CallID      string           `json:"call_id"`
	ContentType string           `json:"content_type"`
	Type        string           `json:"type"`
	Deadline    string           `json:"deadline"`
	Body        string           `json:"body"`
	Protocol    *CallRequestHTTP `json:"protocol"`
}

// jsonOut the expected response from the function container
type jsonOut struct {
	jsonio
	Protocol *CallResponseHTTP `json:"protocol,omitempty"`
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

func writeString(err error, dst io.Writer, str string) error {
	if err != nil {
		return err
	}
	_, err = io.WriteString(dst, str)
	return err
}

// TODO(xxx): headers, query parameters, body - what else should we add to func's payload?
// TODO(xxx): get rid of request body buffering somehow
// @treeder: I don't know why we don't just JSON marshal this, this is rough...
func (h *JSONProtocol) writeJSONToContainer(ci CallInfo) error {
	stdin := json.NewEncoder(h.in)
	bb := new(bytes.Buffer)
	_, err := bb.ReadFrom(ci.Input())
	// todo: better/simpler err handling
	if err != nil {
		return err
	}
	// open
	err = writeString(err, h.in, "{\n")
	if err != nil {
		return err
	}

	// call_id
	err = writeString(err, h.in, `"call_id":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(ci.CallID())
	if err != nil {
		return err
	}

	// content_type
	err = writeString(err, h.in, ",")
	err = writeString(err, h.in, `"content_type":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(ci.ContentType())
	if err != nil {
		return err
	}

	// Call type (sync or async)
	err = writeString(err, h.in, ",")
	err = writeString(err, h.in, `"type":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(ci.CallType())
	if err != nil {
		return err
	}

	// deadline
	err = writeString(err, h.in, ",")
	err = writeString(err, h.in, `"deadline":`)
	if err != nil {
		return err
	}
	err = stdin.Encode(ci.Deadline().String())
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

	// now the extras
	err = writeString(err, h.in, ",")
	err = writeString(err, h.in, `"protocol":{`) // OK name? This is what OpenEvents is calling it in initial proposal
	{
		// Protocol type used to initiate the call.
		err = writeString(err, h.in, `"type":`)
		if err != nil {
			return err
		}
		err = stdin.Encode(ci.ProtocolType())

		// request method
		err = writeString(err, h.in, ",")
		err = writeString(err, h.in, `"method":`)
		if err != nil {
			return err
		}
		err = stdin.Encode(ci.Method())
		if err != nil {
			return err
		}

		// request URL
		err = writeString(err, h.in, ",")
		err = writeString(err, h.in, `"request_url":`)
		if err != nil {
			return err
		}
		err = stdin.Encode(ci.RequestURL())
		if err != nil {
			return err
		}

		// HTTP headers
		err = writeString(err, h.in, ",")
		err = writeString(err, h.in, `"headers":`)
		if err != nil {
			return err
		}
		err = stdin.Encode(ci.Headers())
	}
	err = writeString(err, h.in, "}")

	// close
	err = writeString(err, h.in, "\n}\n\n")
	return err
}

func (h *JSONProtocol) Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error {
	// write input into container
	err := h.writeJSONToContainer(ci)
	if err != nil {
		return err
	}

	// now read the container output
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
		if jout.Protocol != nil {
			p := jout.Protocol
			for k, v := range p.Headers {
				for _, vv := range v {
					rw.Header().Add(k, vv) // on top of any specified on the route
				}
			}
			if p.StatusCode != 0 {
				rw.WriteHeader(p.StatusCode)
			}
		}
		_, err = io.WriteString(rw, jout.Body)
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
