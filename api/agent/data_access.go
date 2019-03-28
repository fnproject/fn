package agent

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/models"
	"github.com/golang/groupcache/singleflight"
	"github.com/patrickmn/go-cache"
)

// ReadDataAccess represents read operations required to operate a load balancer node
type ReadDataAccess interface {
	GetAppID(ctx context.Context, appName string) (string, error)
	// GetAppByID abstracts querying the datastore for an app.
	GetAppByID(ctx context.Context, appID string) (*models.App, error)
	GetTriggerBySource(ctx context.Context, appID string, triggerType, source string) (*models.Trigger, error)
	GetFnByID(ctx context.Context, fnID string) (*models.Fn, error)
}

// XXX(reed): replace all uses of ReadDataAccess with DataAccess or vice versa, whatever is easier
type DataAccess interface {
	ReadDataAccess
}

// CachedDataAccess wraps a DataAccess and caches the results of GetApp.
type cachedDataAccess struct {
	ReadDataAccess

	cache        *cache.Cache
	singleflight singleflight.Group
}

// NewCachedDataAccess is a wrapper that caches entries temporarily
func NewCachedDataAccess(da ReadDataAccess) ReadDataAccess {
	cda := &cachedDataAccess{
		ReadDataAccess: da,
		cache:          cache.New(5*time.Second, 1*time.Minute),
	}
	return cda
}

func appIDCacheKey(appID string) string {
	return "a:" + appID
}

func (da *cachedDataAccess) GetAppID(ctx context.Context, appName string) (string, error) {
	return da.ReadDataAccess.GetAppID(ctx, appName)
}

func (da *cachedDataAccess) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	key := appIDCacheKey(appID)
	app, ok := da.cache.Get(key)
	if ok {
		return app.(*models.App), nil
	}

	resp, err := da.singleflight.Do(key,
		func() (interface{}, error) {
			return da.ReadDataAccess.GetAppByID(ctx, appID)
		})

	if err != nil {
		return nil, err
	}
	app = resp.(*models.App)
	da.cache.Set(key, app, cache.DefaultExpiration)
	return app.(*models.App), nil
}
