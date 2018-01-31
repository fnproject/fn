package protocol

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
)

var errInvalidProtocol = errors.New("Invalid Protocol")

type errorProto struct {
	error
}

func (e errorProto) IsStreamable() bool                                           { return false }
func (e errorProto) Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error { return e }

// ContainerIO defines the interface used to talk to a hot function.
// Internally, a protocol must know when to alternate between stdin and stdout.
// It returns any protocol error, if present.
type ContainerIO interface {
	IsStreamable() bool

	// Dispatch will handle sending stdin and stdout to a container. Implementers
	// of Dispatch may format the input and output differently. Dispatch must respect
	// the req.Context() timeout / cancellation.
	Dispatch(ctx context.Context, ci CallInfo, w io.Writer) error
}

// CallInfo is passed into dispatch with only the required data the protocols require
type CallInfo interface {
	CallID() string
	ContentType() string
	Input() io.Reader
	Deadline() strfmt.DateTime
	CallType() string

	// ProtocolType let's function/fdk's know what type original request is. Only 'http' for now.
	// This could be abstracted into separate Protocol objects for each type and all the following information could go in there.
	// This is a bit confusing because we also have the protocol's for getting information in and out of the function containers.
	ProtocolType() string
	Request() *http.Request
	Method() string
	RequestURL() string
	Headers() map[string][]string
}

type callInfoImpl struct {
	call *models.Call
	req  *http.Request
}

func (ci callInfoImpl) CallID() string {
	return ci.call.ID
}

func (ci callInfoImpl) ContentType() string {
	return ci.req.Header.Get("Content-Type")
}

// Input returns the call's input/body
func (ci callInfoImpl) Input() io.Reader {
	return ci.req.Body
}

func (ci callInfoImpl) Deadline() strfmt.DateTime {
	deadline, ok := ci.req.Context().Deadline()
	if !ok {
		// In theory deadline must have been set here, but if it wasn't then
		// at this point it is already too late to raise an error. Set it to
		// something meaningful.
		// This assumes StartedAt was set to something other than the default.
		// If that isn't set either, then how many things have gone wrong?
		if ci.call.StartedAt == strfmt.NewDateTime() {
			// We just panic if StartedAt is the default (i.e. not set)
			panic("No context deadline and zero-value StartedAt - this should never happen")
		}
		deadline = ((time.Time)(ci.call.StartedAt)).Add(time.Duration(ci.call.Timeout) * time.Second)
	}
	return strfmt.DateTime(deadline)
}

// CallType returns whether the function call was "sync" or "async".
func (ci callInfoImpl) CallType() string {
	return ci.call.Type
}

// ProtocolType at the moment can only be "http". Once we have Kafka or other
// possible origins for calls this will track what the origin was.
func (ci callInfoImpl) ProtocolType() string {
	return "http"
}

// Request basically just for the http format, since that's the only that makes sense to have the full request as is
func (ci callInfoImpl) Request() *http.Request {
	return ci.req
}
func (ci callInfoImpl) Method() string {
	return ci.call.Method
}
func (ci callInfoImpl) RequestURL() string {
	return ci.call.URL
}
func (ci callInfoImpl) Headers() map[string][]string {
	return ci.req.Header
}

func NewCallInfo(call *models.Call, req *http.Request) CallInfo {
	ci := &callInfoImpl{
		call: call,
		req:  req,
	}
	return ci
}

// Protocol defines all protocols that operates a ContainerIO.
type Protocol string

// hot function protocols
const (
	Default Protocol = models.FormatDefault
	HTTP    Protocol = models.FormatHTTP
	JSON    Protocol = models.FormatJSON
	Empty   Protocol = ""
)

func (p *Protocol) UnmarshalJSON(b []byte) error {
	switch Protocol(b) {
	case Empty, Default:
		*p = Default
	case HTTP:
		*p = HTTP
	case JSON:
		*p = JSON
	default:
		return errInvalidProtocol
	}
	return nil
}

func (p Protocol) MarshalJSON() ([]byte, error) {
	switch p {
	case Empty, Default:
		return []byte(Default), nil
	case HTTP:
		return []byte(HTTP), nil
	case JSON:
		return []byte(JSON), nil
	}
	return nil, errInvalidProtocol
}

// New creates a valid protocol handler from a I/O pipe representing containers
// stdin/stdout.
func New(p Protocol, in io.Writer, out io.Reader) ContainerIO {
	switch p {
	case HTTP:
		return &HTTPProtocol{in, out}
	case JSON:
		return &JSONProtocol{in, out}
	case Default, Empty:
		return &DefaultProtocol{}
	}
	return &errorProto{errInvalidProtocol}
}

// IsStreamable says whether the given protocol can be used for streaming into
// hot functions.
func IsStreamable(p Protocol) bool { return New(p, nil, nil).IsStreamable() }
