package datastore

import (
	"context"
	"net/url"

	"fmt"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

func New(ctx context.Context, dbURL string) (models.Datastore, error) {
	log := common.Logger(ctx)
	u, err := url.Parse(dbURL)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	log.WithFields(logrus.Fields{"db": u.Scheme}).Debug("creating new datastore")

	for _, provider := range providers {
		if provider.Supports(u) {
			return provider.New(ctx, u)
		}
	}
	return nil, fmt.Errorf("no data store provider found for storage url %s", u)
}

func Wrap(ds models.Datastore) models.Datastore {
	return datastoreutil.MetricDS(datastoreutil.NewValidator(ds))
}

type Provider interface {
	Supports(url *url.URL) bool
	New(ctx context.Context, url *url.URL) (models.Datastore, error)
}

var providers []Provider

func AddProvider(provider Provider) {
	providers = append(providers, provider)
}
