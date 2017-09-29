package protocol

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// JSONIn is what's sent into the function
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

	// convert to JSON func format
	jin := &JSONIO{
		Headers: req.Header,
		Body:    body.String(),
	}
	b, err := json.Marshal(jin)
	if err != nil {
		// this shouldn't happen
		return respondWithError(
			w, fmt.Errorf("error marshalling JSONInput: %s", err.Error()))
	}
	// TODO: write in chunks, how big should chunk be?
	_, err = h.in.Write(b)
	if err != nil {
		return respondWithError(
			w, fmt.Errorf("error writing JSON object to function's STDIN: %s", err.Error()))
	}

	if rw, ok := w.(http.ResponseWriter); ok {
		// this has to be done for pulling out:
		// - status code
		// - body
		jout := new(JSONIO)
		dec := json.NewDecoder(h.out)
		if err := dec.Decode(jout); err != nil {
			return respondWithError(
				w, fmt.Errorf("unable to decode JSON response object: %s", err.Error()))
		}
		rw.WriteHeader(jout.StatusCode)
		_, err = rw.Write([]byte(jout.Body)) // TODO timeout
		if err != nil {
			return respondWithError(
				w, fmt.Errorf("unable to write JSON response object: %s", err.Error()))
		}
	} else {
		// logs can just copy the full thing in there, headers and all.
		_, err = io.Copy(w, bufio.NewReader(h.out))
		if err != nil {
			return respondWithError(
				w, fmt.Errorf("error reading function response: %s", err.Error()))
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
