package cache

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/common/singleflight"
	"github.com/fnproject/fn/api/models"
	"github.com/patrickmn/go-cache"
)

type cacheDB struct {
	models.Datastore

	cache        *cache.Cache
	singleflight singleflight.SingleFlight // singleflight assists Datastore
}

// Wrap implements models.Datastore by wrapping an existing datastore and
// adding caching around certain methods. At present, GetApp and GetRoute add
// caching.
func Wrap(ds models.Datastore) models.Datastore {
	return &cacheDB{
		Datastore: ds,
		cache:     cache.New(5*time.Second, 1*time.Minute), // TODO configurable from env
	}
}

func (c *cacheDB) GetApp(ctx context.Context, appName string) (*models.App, error) {
	key := appCacheKey(appName)
	app, ok := c.cache.Get(key)
	if ok {
		return app.(*models.App), nil
	}

	resp, err := c.singleflight.Do(key,
		func() (interface{}, error) { return c.Datastore.GetApp(ctx, appName) },
	)
	if err != nil {
		return nil, err
	}
	app = resp.(*models.App)
	c.cache.Set(key, app, cache.DefaultExpiration)
	return app.(*models.App), nil
}



func (c *cacheDB) MatchRoute(ctx context.Context, appName, path string) (*models.Route, error) {
	key := routeCacheKey(appName, path)
	route, ok := c.cache.Get(key)
	if ok {
		return route.(*models.Route), nil
	}

	resp, err := c.singleflight.Do(key,
		func() (interface{}, error) { return c.Datastore.MatchRoute(ctx, appName, path) },
	)
	if err != nil {
		return nil, err
	}
	route = resp.(*models.Route)
	c.cache.Set(key, route, cache.DefaultExpiration)
	return route.(*models.Route), nil
}

func routeCacheKey(appname, path string) string {
	return "r:" + appname + "\x00" + path
}

func appCacheKey(appname string) string {
	return "a:" + appname
}
