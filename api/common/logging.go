package common

import (
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func SetLogFormat(format string) {
	if format != "text" && format != "json" {
		logrus.WithFields(logrus.Fields{"format": format}).Warn("Unknown log format specified, using text. Possible options are json and text.")
	}

	if format == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	} else {
		// show full timestamps
		formatter := &logrus.TextFormatter{
			FullTimestamp: true,
		}
		logrus.SetFormatter(formatter)
	}
}

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

	// this effectively just adds more gin log goodies
	gin.SetMode(gin.ReleaseMode)
	if logLevel == logrus.DebugLevel {
		gin.SetMode(gin.DebugMode)
	}
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

	parsed, err := url.Parse(to)
	if parsed.Host == "" && parsed.Path == "" {
		logrus.WithFields(logrus.Fields{"to": to}).Warn("No scheme on logging url, adding udp://")
		// this happens when no scheme like udp:// is present
		to = "udp://" + to
		parsed, err = url.Parse(to)
	}
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"to": to}).Error("could not parse logging URI, defaulting to stderr")
		return
	}

	// File URL must contain only `url.Path`. Syslog location must contain only `url.Host`
	if (parsed.Host == "" && parsed.Path == "") || (parsed.Host != "" && parsed.Path != "") {
		logrus.WithFields(logrus.Fields{"to": to, "uri": parsed}).Error("invalid logging location, defaulting to stderr")
		return
	}

	switch parsed.Scheme {
	case "udp", "tcp":
		err = NewSyslogHook(parsed, prefix)
		if err != nil {
			logrus.WithFields(logrus.Fields{"uri": parsed, "to": to}).WithError(err).Error("unable to connect to syslog, defaulting to stderr")
			return
		}
	case "file":
		f, err := os.OpenFile(parsed.Path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{"to": to, "path": parsed.Path}).Error("cannot open file, defaulting to stderr")
			return
		}
		logrus.SetOutput(f)
	default:
		logrus.WithFields(logrus.Fields{"scheme": parsed.Scheme, "to": to}).Error("unknown logging location scheme, defaulting to stderr")
	}
}

// MaskPassword returns a stringified URL without its password visible
func MaskPassword(u *url.URL) string {
	if u.User != nil {
		p, set := u.User.Password()
		if set {
			return strings.Replace(u.String(), p+"@", "***@", 1)
		}
	}
	return u.String()
}
