package datastore

import (
	"context"
	"fmt"
	"net/url"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/datastore/sql"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

func New(ctx context.Context, dbURL string) (models.Datastore, error) {
	ds, err := newds(ctx, dbURL) // teehee
	if err != nil {
		return nil, err
	}

	return Wrap(ds), nil
}

func Wrap(ds models.Datastore) (models.Datastore) {
	return datastoreutil.MetricDS(datastoreutil.NewValidator(ds))
}

func newds(ctx context.Context, dbURL string) (models.Datastore, error) {
	log := common.Logger(ctx)
	u, err := url.Parse(dbURL)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	log.WithFields(logrus.Fields{"db": u.Scheme}).Debug("creating new datastore")
	switch u.Scheme {
	case "sqlite3", "postgres", "mysql":
		return sql.New(ctx, u)
	default:
		return nil, fmt.Errorf("db type not supported %v", u.Scheme)
	}
}
