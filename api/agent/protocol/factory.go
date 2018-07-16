package protocol

import (
	"context"
	"errors"
	"io"

	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/models"
)

var errInvalidProtocol = errors.New("Invalid Protocol")

var ErrExcessData = errors.New("Excess data in stream")

type errorProto struct {
	error
}

func (e errorProto) IsStreamable() bool { return false }
func (e errorProto) Dispatch(ctx context.Context, ci *event.Event, w io.Writer) (*event.Event, error) {
	return nil, e
}

// ContainerIO defines the interface used to talk to a hot function.
// Internally, a protocol must know when to alternate between stdin and stdout.
// It returns any protocol error, if present.
type ContainerIO interface {
	IsStreamable() bool

	// Dispatch dispatches an event to a container and handles/parses the response
	// an error response indicates that the container has reached an invalid state and should be discarded
	Dispatch(ctx context.Context, evt *event.Event) (*event.Event, error)
}

// Protocol defines all protocols that operates a ContainerIO.
type Protocol string

// hot function protocols
const (
	Default     Protocol = models.FormatDefault
	HTTP        Protocol = models.FormatHTTP
	JSON        Protocol = models.FormatJSON
	CloudEventP Protocol = models.FormatCloudEvent
	Empty       Protocol = ""
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
		return &httpProtocol{in, out}
	case JSON:
		return &JSONProtocol{in, out}
	case CloudEventP:
		return &cloudEventProtocol{in, out}
	case Default, Empty:
		return &DefaultProtocol{}
	}
	return &errorProto{errInvalidProtocol}
}

// IsStreamable says whether the given protocol can be used for streaming into
// hot functions.
func IsStreamable(p Protocol) bool { return New(p, nil, nil).IsStreamable() }
