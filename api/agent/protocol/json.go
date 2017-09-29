package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func (h *JSONProtocol) Dispatch(w io.Writer, req *http.Request) error {
	var body bytes.Buffer
	if req.Body != nil {
		var dest io.Writer = &body

		// TODO copy w/ ctx
		_, err := io.Copy(dest, req.Body)
		if err != nil {
			return respondWithError(
				w, fmt.Errorf("error reader JSON object from request body: %s", err.Error()))
		}
		defer req.Body.Close()
	}
	err := json.NewEncoder(h.in).Encode(&JSONIO{
		Headers: req.Header,
		Body:    body.String(),
	})
	if err != nil {
		// this shouldn't happen
		return respondWithError(
			w, fmt.Errorf("error marshalling JSONInput: %s", err.Error()))
	}

	jout := new(JSONIO)
	dec := json.NewDecoder(h.out)
	if err := dec.Decode(jout); err != nil {
		return respondWithError(
			w, fmt.Errorf("unable to decode JSON response object: %s", err.Error()))
	}
	if rw, ok := w.(http.ResponseWriter); ok {
		// this has to be done for pulling out:
		// - status code
		// - body
		rw.WriteHeader(jout.StatusCode)
		_, err = rw.Write([]byte(jout.Body)) // TODO timeout
		if err != nil {
			return respondWithError(
				w, fmt.Errorf("unable to write JSON response object: %s", err.Error()))
		}
	} else {
		// logs can just copy the full thing in there, headers and all.
		err = json.NewEncoder(w).Encode(jout)
		if err != nil {
			return respondWithError(
				w, fmt.Errorf("error writing function response: %s", err.Error()))
		}
	}
	return nil
}

func respondWithError(w io.Writer, err error) error {
	errMsg := []byte(err.Error())
	statusCode := http.StatusInternalServerError
	if rw, ok := w.(http.ResponseWriter); ok {
		rw.WriteHeader(statusCode)
		rw.Write(errMsg)
	} else {
		// logs can just copy the full thing in there, headers and all.
		w.Write(errMsg)
	}

	return err
}
