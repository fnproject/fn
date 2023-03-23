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

// New creates a DataStore from the specified URL
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

// Provider is a datastore provider
type Provider interface {
	fmt.Stringer
	// Supports indicates if this provider can handle a given data store.
	Supports(url *url.URL) bool
	// New creates a new data store from the specified URL
	New(ctx context.Context, url *url.URL) (models.Datastore, error)
}

var providers []Provider

// Register globally registers a data store provider
func Register(provider Provider) {
	//logrus.Infof("Registering data store provider '%s'", provider)
	fmt.Printf("Registering data store provider '%s'\n", provider)
	providers = append(providers, provider)
}
