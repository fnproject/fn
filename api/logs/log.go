package logs

import (
	"fmt"
	"net/url"

	"context"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/logs/metrics"
	"github.com/fnproject/fn/api/logs/validator"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

// Provider defines a source that can create log stores
type Provider interface {
	fmt.Stringer
	// Supports indicates if this provider can handle a specific URL scheme
	Supports(url *url.URL) bool
	//Create a new log store from the corresponding URL
	New(ctx context.Context, url *url.URL) (models.LogStore, error)
}

var providers []Provider

// AddProvider globally registers a new LogStore provider
func AddProvider(pf Provider) {
	logrus.Infof("Adding log provider %s", pf)
	providers = append(providers, pf)
}

// New Creates a new log store based on a given URL
func New(ctx context.Context, dbURL string) (models.LogStore, error) {
	log := common.Logger(ctx)
	u, err := url.Parse(dbURL)
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{"url": dbURL}).Fatal("bad DB URL")
	}
	log.WithFields(logrus.Fields{"db": u.Scheme}).Debug("creating log store")

	for _, p := range providers {
		if p.Supports(u) {
			return p.New(ctx, u)
		}
	}
	return nil, fmt.Errorf("no log store provider available for url %s", dbURL)
}

func Wrap(ls models.LogStore) models.LogStore {
	return validator.NewValidator(metrics.NewLogstore(ls))
}
