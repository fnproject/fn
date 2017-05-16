package datastore

import (
	"fmt"
	"net/url"

	"github.com/Sirupsen/logrus"
	"gitlab.oracledx.com/odx/functions/api/datastore/bolt"
	"gitlab.oracledx.com/odx/functions/api/datastore/mysql"
	"gitlab.oracledx.com/odx/functions/api/datastore/postgres"
	"gitlab.oracledx.com/odx/functions/api/datastore/redis"
	"gitlab.oracledx.com/odx/functions/api/models"
)

func New(dbURL string) (models.Datastore, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	logrus.WithFields(logrus.Fields{"db": u.Scheme}).Debug("creating new datastore")
	switch u.Scheme {
	case "bolt":
		return bolt.New(u)
	case "postgres":
		return postgres.New(u)
	case "mysql":
		return mysql.New(u)
	case "redis":
		return redis.New(u)
	default:
		return nil, fmt.Errorf("db type not supported %v", u.Scheme)
	}
}
