package datastore

import (
	"context"
	"fmt"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

// New creates a DataStore from the specified URL
func New(ctx context.Context, dbURL string) (models.Datastore, error) {
	log := common.Logger(ctx)
	log.WithFields(logrus.Fields{"db": dbURL}).Debug("creating new datastore")

	for _, provider := range providers {
		if provider.Supports(dbURL) {
			return provider.New(ctx, dbURL)
		}
	}
	return nil, fmt.Errorf("no data store provider found for storage url %s", dbURL)
}

func Wrap(ds models.Datastore) models.Datastore {
	return datastoreutil.MetricDS(datastoreutil.NewValidator(ds))
}

// Provider is a datastore provider
type Provider interface {
	fmt.Stringer
	// Supports indicates if this provider can handle a given data store.
	Supports(url string) bool
	// New creates a new data store from the specified URL
	New(ctx context.Context, url string) (models.Datastore, error)
}

var providers []Provider

// Register globally registers a data store provider
func Register(provider Provider) {
	logrus.Infof("Registering data store provider '%s'", provider)
	providers = append(providers, provider)
}
