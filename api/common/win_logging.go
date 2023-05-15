//go:build !linux && !darwin
// +build !linux,!darwin

package common

import (
	"errors"
	"net/url"
)

func NewSyslogHook(url *url.URL, prefix string) error {
	return errors.New("Syslog not supported on this system.")
}
