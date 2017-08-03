// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
