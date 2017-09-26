package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// JSONInput is what's sent into the function
// All HTTP request headers should be set in env
type JSONInput struct {
	Body string `json:"body"`
}

// JSONOutput function must return this format
// StatusCode value must be a HTTP status code
type JSONOutput struct {
	StatusCode int    `json:"status"`
	Body       string `json:"body"`
}

// JSONProtocol converts stdin/stdout streams from HTTP into JSON format.
type JSONProtocol struct {
	in  io.Writer
	out io.Reader
}

func (p *JSONProtocol) IsStreamable() bool {
	return true
}

type Error struct {
	Message string `json:"message"`
}

type ErrMsg struct {
	Err Error `json:"error"`
}

func (h *JSONProtocol) Dispatch(w io.Writer, req *http.Request) error {
	var body bytes.Buffer
	if req.Body != nil {
		var dest io.Writer = &body

		// TODO copy w/ ctx
		nBytes, _ := strconv.ParseInt(
			req.Header.Get("Content-Length"), 10, 64)
		_, err := io.Copy(dest, io.LimitReader(req.Body, nBytes))
		if err != nil {
			respondWithError(w, err)
			return err
		}
	}

	// convert to JSON func format
	jin := &JSONInput{
		Body: body.String(),
	}
	b, err := json.Marshal(jin)
	if err != nil {
		// this shouldn't happen
		err = fmt.Errorf("error marshalling JSONInput: %v", err)
		respondWithError(w, err)
		return err
	}
	h.in.Write(b)

	maxContentSize := int64(1 * 1024 * 1024) // 1Mb should be enough
	jout := &JSONOutput{}
	dec := json.NewDecoder(io.LimitReader(h.out, maxContentSize))
	if err := dec.Decode(jout); err != nil {
		err = fmt.Errorf("Unable to decode JSON response object: %s", err.Error())
		respondWithError(w, err)
		return err
	}

	if rw, ok := w.(http.ResponseWriter); ok {
		b, err = json.Marshal(jout.Body)
		if err != nil {
			err = fmt.Errorf("error unmarshalling JSON body: %s", err.Error())
			respondWithError(w, err)
			return err
		}
		rw.WriteHeader(jout.StatusCode)
		rw.Write(b) // TODO timeout
	} else {
		// logs can just copy the full thing in there, headers and all.
		b, err = json.Marshal(jout)
		if err != nil {
			err = fmt.Errorf("error unmarshalling JSON response: %s", err.Error())
			respondWithError(w, err)
			return err
		}
		w.Write(b) // TODO timeout
	}
	return nil
}

func respondWithError(w io.Writer, err error) {
	errMsg := ErrMsg{
		Err: Error{
			Message: err.Error(),
		},
	}
	b, _ := json.Marshal(errMsg)
	statusCode := 500
	writeResponse(w, b, statusCode)
}

func writeResponse(w io.Writer, b []byte, statusCode int) {
	if rw, ok := w.(http.ResponseWriter); ok {
		rw.WriteHeader(statusCode)
		rw.Write(b) // TODO timeout
	} else {
		// logs can just copy the full thing in there, headers and all.
		w.Write(b) // TODO timeout
	}
}
