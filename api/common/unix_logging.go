//go:build !windows && !nacl && !plan9
// +build !windows,!nacl,!plan9

package common

import (
	"io/ioutil"
	"net/url"

	"github.com/sirupsen/logrus"
	logrus_syslog "github.com/sirupsen/logrus/hooks/syslog"
)

func NewSyslogHook(url *url.URL, prefix string) error {
	syslog, err := logrus_syslog.NewSyslogHook(url.Scheme, url.Host, 0, prefix)
	if err != nil {
		return err
	}
	logrus.AddHook(syslog)
	// TODO we could support multiple destinations...
	logrus.SetOutput(ioutil.Discard)
	return nil
}
