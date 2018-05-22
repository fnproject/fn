package logs

import (
	"fmt"
	"net/url"

	"context"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore/sql"
	"github.com/fnproject/fn/api/logs/metrics"
	"github.com/fnproject/fn/api/logs/s3"
	"github.com/fnproject/fn/api/logs/validator"
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
	var ls models.LogStore
	switch u.Scheme {
	case "sqlite3", "postgres", "mysql":
		ls, err = sql.New(ctx, u)
	case "s3":
		ls, err = s3.New(u)
	default:
		err = fmt.Errorf("db type not supported %v", u.Scheme)
	}

	return ls, err
}

func Wrap(ls models.LogStore) models.LogStore {
	return validator.NewValidator(metrics.NewLogstore(ls))
}
