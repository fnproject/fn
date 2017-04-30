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
	"net/url"
	"os"

	"github.com/Sirupsen/logrus"
)

func SetLogLevel(ll string) {
	if ll == "" {
		ll = "info"
	}
	logrus.WithFields(logrus.Fields{"level": ll}).Info("Setting log level to")
	logLevel, err := logrus.ParseLevel(ll)
	if err != nil {
		logrus.WithFields(logrus.Fields{"level": ll}).Warn("Could not parse log level, setting to INFO")
		logLevel = logrus.InfoLevel
	}
	logrus.SetLevel(logLevel)
}

func SetLogDest(to, prefix string) {
	logrus.SetOutput(os.Stderr) // in case logrus changes their mind...
	if to == "stderr" {
		return
	}

	// possible schemes: { udp, tcp, file }
	// file url must contain only a path, syslog must contain only a host[:port]
	// expect: [scheme://][host][:port][/path]
	// default scheme to udp:// if none given

	url, err := url.Parse(to)
	if url.Host == "" && url.Path == "" {
		logrus.WithFields(logrus.Fields{"to": to}).Warn("No scheme on logging url, adding udp://")
		// this happens when no scheme like udp:// is present
		to = "udp://" + to
		url, err = url.Parse(to)
	}
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"to": to}).Error("could not parse logging URI, defaulting to stderr")
		return
	}

	// File URL must contain only `url.Path`. Syslog location must contain only `url.Host`
	if (url.Host == "" && url.Path == "") || (url.Host != "" && url.Path != "") {
		logrus.WithFields(logrus.Fields{"to": to, "uri": url}).Error("invalid logging location, defaulting to stderr")
		return
	}

	switch url.Scheme {
	case "udp", "tcp":
		err = NewSyslogHook(url, prefix)
		if err != nil {
			logrus.WithFields(logrus.Fields{"uri": url, "to": to}).WithError(err).Error("unable to connect to syslog, defaulting to stderr")
			return
		}
	case "file":
		f, err := os.OpenFile(url.Path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"to": to, "path": url.Path}).Error("cannot open file, defaulting to stderr")
			return
		}
		logrus.SetOutput(f)
	default:
		logrus.WithFields(logrus.Fields{"scheme": url.Scheme, "to": to}).Error("unknown logging location scheme, defaulting to stderr")
	}
}
