package datastore

import (
	"fmt"
	"net/url"

	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/datastore/sql"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

func New(dbURL string) (models.Datastore, error) {
	ds, err := newds(dbURL) // teehee
	if err != nil {
		return nil, err
	}

	return datastoreutil.MetricDS(datastoreutil.NewValidator(ds)), nil
}

func newds(dbURL string) (models.Datastore, error) {
	u, err := url.Parse(dbURL)
	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	logrus.WithFields(logrus.Fields{"db": u.Scheme}).Debug("creating new datastore")
	switch u.Scheme {
	case "sqlite3", "postgres", "mysql":
		return sql.New(u)
	default:
		return nil, fmt.Errorf("db type not supported %v", u.Scheme)
	}
}
