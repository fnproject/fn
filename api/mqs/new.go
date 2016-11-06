package mqs

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/models"
)

// New will parse the URL and return the correct MQ implementation.
func New(mqURL string) (models.MessageQueue, error) {
	// Play with URL schemes here: https://play.golang.org/p/xWAf9SpCBW
	u, err := url.Parse(mqURL)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": mqURL}).Fatal("bad MQ URL")
	}
	logrus.WithFields(logrus.Fields{"mq": u.Scheme}).Debug("selecting MQ")
	switch u.Scheme {
	case "memory":
		return NewMemoryMQ(), nil
	case "redis":
		return NewRedisMQ(u)
	case "bolt":
		return NewBoltMQ(u)
	}
	if strings.HasPrefix(u.Scheme, "ironmq") {
		return NewIronMQ(u), nil
	}

	return nil, fmt.Errorf("mq type not supported %v", u.Scheme)
}
