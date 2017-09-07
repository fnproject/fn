package common

import (
	"io"
	"net"
	"syscall"
)

type Temporary interface {
	Temporary() bool
}

func IsTemporary(err error) bool {
	v, ok := err.(Temporary)
	return (ok && v.Temporary()) || isNet(err)
}

func isNet(err error) bool {
	if _, ok := err.(net.Error); ok {
		return true
	}

	switch err := err.(type) {
	case *net.OpError:
		return true
	case syscall.Errno:
		if err == syscall.ECONNREFUSED { // linux only? maybe ok for prod
			return true // connection refused
		}
	default:
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			return true
		}
	}
	return false
}
