package datastore

import (
	"fmt"
	"net/url"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/datastore/bolt"
	"github.com/iron-io/functions/api/datastore/postgres"
	"github.com/iron-io/functions/api/models"
)

func New(dbURL string) (models.Datastore, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		logrus.WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	logrus.WithFields(logrus.Fields{"db": u.Scheme}).Info("creating new datastore")
	switch u.Scheme {
	case "bolt":
		return bolt.New(u)
	case "postgres":
		return postgres.New(u)
	default:
		return nil, fmt.Errorf("db type not supported %v", u.Scheme)
	}
}
