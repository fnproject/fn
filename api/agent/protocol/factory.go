package protocol

import (
	"errors"
	"io"
	"net/http"

	"github.com/fnproject/fn/api/models"
)

var errInvalidProtocol = errors.New("Invalid Protocol")

type errorProto struct {
	error
}

func (e errorProto) IsStreamable() bool                                    { return false }
func (e errorProto) Dispatch(*models.Call, io.Writer, *http.Request) error { return e }

// ContainerIO defines the interface used to talk to a hot function.
// Internally, a protocol must know when to alternate between stdin and stdout.
// It returns any protocol error, if present.
type ContainerIO interface {
	IsStreamable() bool

	// Dispatch will handle sending stdin and stdout to a container. Implementers
	// of Dispatch may format the input and output differently. Dispatch must respect
	// the req.Context() timeout / cancellation.
	Dispatch(call *models.Call, w io.Writer, req *http.Request) error
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
