package logs

import (
	"fmt"
	"net/url"

	"context"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore/sql"
	"github.com/fnproject/fn/api/logs/s3"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

func New(ctx context.Context, dbURL string) (models.LogStore, error) {
	log := common.Logger(ctx)
	u, err := url.Parse(dbURL)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	log.WithFields(logrus.Fields{"db": u.Scheme}).Debug("creating log store")
	switch u.Scheme {
	case "sqlite3", "postgres", "mysql":
		return sql.New(ctx, u)
	case "s3":
		return s3.New(u)
	default:
		return nil, fmt.Errorf("db type not supported %v", u.Scheme)
	}
}
