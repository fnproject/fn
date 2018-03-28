package models

import (
	"context"
	"io"
	"net/http"
)

func reqURL(req *http.Request) string {
	if req.URL.Scheme == "" {
		if req.TLS == nil {
			req.URL.Scheme = "http"
		} else {
			req.URL.Scheme = "https"
		}
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	return req.URL.String()
}

type EventRequest struct {
	ctx         context.Context
	contentType string
	header      http.Header
	body        io.Reader
	requestPath string
	method      string
	eventType   string

	// required by HTTP protocol
	// we can't do a copy of the request because it will turn
	// the event into yet another HTTP request-like struct,
	// so just keeping the reference to that
	OriginalRequest *http.Request
}

func (e *EventRequest) FromHTTPRequest(req *http.Request) {
	e.ctx = req.Context()
	e.header = req.Header
	e.body = req.Body
	e.requestPath = reqURL(req)
	e.method = req.Method

	e.eventType = "http"
	e.OriginalRequest = req
	e.contentType = req.Header.Get("Content-Type")
}

func (e *EventRequest) Context() context.Context {
	return e.ctx
}

func (e *EventRequest) WithContext(ctx context.Context) *EventRequest {
	e.ctx = ctx
	return e
}

func (e *EventRequest) Header() http.Header {
	return e.header
}

func (e *EventRequest) WithHeader(h http.Header) {
	e.header = h
}

func (e *EventRequest) Body() io.Reader {
	return e.body
}

func (e *EventRequest) WithBody(b io.Reader) {
	e.body = b
}

func (e *EventRequest) WithRequestURL(u string) {
	e.requestPath = u
}

func (e *EventRequest) RequestURL() string {
	return e.requestPath
}

func (e *EventRequest) Method() string {
	return e.method
}

func (e *EventRequest) WithMethod(m string) {
	e.method = m
}

func (e *EventRequest) Type() string {
	return e.eventType
}

func (e *EventRequest) ContentType() string {
	if e.contentType == "" {
		e.contentType = e.header.Get("Content-Type")
	}
	return e.contentType
}
