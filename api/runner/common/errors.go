package common

import (
	"io"
	"net"
	"syscall"
)

// Errors that can be directly exposed to task creators/users.
type UserVisibleError interface {
	UserVisible() bool
}

func IsUserVisibleError(err error) bool {
	ue, ok := err.(UserVisibleError)
	return ok && ue.UserVisible()
}

type userVisibleError struct {
	error
}

func (u *userVisibleError) UserVisible() bool { return true }

func UserError(err error) error {
	return &userVisibleError{err}
}

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
