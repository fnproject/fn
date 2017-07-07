package logs

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"gitlab-odx.oracle.com/odx/functions/api/models"
	"net/url"
)

func New(dbURL string) (models.FnLog, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	logrus.WithFields(logrus.Fields{"db": u.Scheme}).Debug("creating new datastore")
	switch u.Scheme {
	case "bolt":
		return NewBolt(u)
	default:
		return nil, fmt.Errorf("db type not supported %v", u.Scheme)
	}
}
