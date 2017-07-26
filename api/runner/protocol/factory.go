package protocol

import (
	"context"
	"errors"
	"io"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/task"
)

var errInvalidProtocol = errors.New("Invalid Protocol")

// ContainerIO defines the interface used to talk to a hot function.
// Internally, a protocol must know when to alternate between stdin and stdout.
// It returns any protocol error, if present.
type ContainerIO interface {
	IsStreamable() bool
	Dispatch(ctx context.Context, t task.Request) error
}

// Protocol defines all protocols that operates a ContainerIO.
type Protocol string

// hot function protocols
const (
	Default Protocol = models.FormatDefault
	HTTP    Protocol = models.FormatHTTP
	Empty   Protocol = ""
)

func (p *Protocol) UnmarshalJSON(b []byte) error {
	switch Protocol(b) {
	case Empty, Default:
		*p = Default
	case HTTP:
		*p = HTTP
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
	}
	return nil, errInvalidProtocol
}

// implements ContainerIO
type errorProto struct{}

func (e *errorProto) IsStreamable() bool                                 { return false }
func (e *errorProto) Dispatch(ctx context.Context, t task.Request) error { return errInvalidProtocol }

// New creates a valid protocol handler from a I/O pipe representing containers
// stdin/stdout.
func New(p Protocol, in io.Writer, out io.Reader) ContainerIO {
	switch p {
	case HTTP:
		return &HTTPProtocol{in, out}
	case Default, Empty:
		return &DefaultProtocol{}
	}
	return &errorProto{} // shouldn't make it past testing...
}

// IsStreamable says whether the given protocol can be used for streaming into
// hot functions.
// TODO get rid of ContainerIO and just use Protocol
func IsStreamable(p Protocol) bool {
	return New(p, nil, nil).IsStreamable()
}
