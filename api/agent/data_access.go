package agent

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/models"
	"github.com/golang/groupcache/singleflight"
	"github.com/patrickmn/go-cache"
	"go.opencensus.io/trace"
)

// XXX(reed): this is only used by the front end now, this should be in the server/ package

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

// NewMetricReadDataAccess adds metrics to a ReadDataAccess
func NewMetricReadDataAccess(rda ReadDataAccess) ReadDataAccess {
	return &metricda{rda}
}

type metricda struct {
	rda ReadDataAccess
}

func (m *metricda) GetTriggerBySource(ctx context.Context, appID string, triggerType, source string) (*models.Trigger, error) {
	ctx, span := trace.StartSpan(ctx, "rda_get_trigger_by_source")
	defer span.End()
	return m.rda.GetTriggerBySource(ctx, appID, triggerType, source)
}

func (m *metricda) GetAppID(ctx context.Context, appName string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "rda_get_app_id")
	defer span.End()
	return m.rda.GetAppID(ctx, appName)
}

func (m *metricda) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	ctx, span := trace.StartSpan(ctx, "rda_get_app_by_id")
	defer span.End()
	return m.rda.GetAppByID(ctx, appID)
}

func (m *metricda) GetFnByID(ctx context.Context, fnID string) (*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "rda_get_fn_by_id")
	defer span.End()
	return m.rda.GetFnByID(ctx, fnID)
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

func appIDCacheKey(appID string) string     { return "a:" + appID }
func appNameCacheKey(appName string) string { return "n:" + appName }
func fnCacheKey(fnID string) string         { return "f:" + fnID }
func trigSourceCacheKey(app, typ, source string) string {
	return "t:" + app + string('\x00') + typ + string('\x00') + source
}

func (da *cachedDataAccess) GetAppID(ctx context.Context, appName string) (string, error) {
	key := appNameCacheKey(appName)
	app, ok := da.cache.Get(key)
	if ok {
		return app.(string), nil
	}

	resp, err := da.singleflight.Do(key,
		func() (interface{}, error) {
			return da.ReadDataAccess.GetAppID(ctx, appName)
		})

	if err != nil {
		return "", err
	}
	app = resp.(string)
	da.cache.Set(key, app, cache.DefaultExpiration)
	return app.(string), nil
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

func (da *cachedDataAccess) GetTriggerBySource(ctx context.Context, appID string, triggerType, source string) (*models.Trigger, error) {
	key := trigSourceCacheKey(appID, triggerType, source)
	trigger, ok := da.cache.Get(key)
	if ok {
		return trigger.(*models.Trigger), nil
	}

	resp, err := da.singleflight.Do(key,
		func() (interface{}, error) {
			return da.ReadDataAccess.GetTriggerBySource(ctx, appID, triggerType, source)
		})

	if err != nil {
		return nil, err
	}
	trigger = resp.(*models.Trigger)
	da.cache.Set(key, trigger, cache.DefaultExpiration)
	return trigger.(*models.Trigger), nil
}

func (da *cachedDataAccess) GetFnByID(ctx context.Context, fnID string) (*models.Fn, error) {
	key := fnCacheKey(fnID)
	fn, ok := da.cache.Get(key)
	if ok {
		return fn.(*models.Fn), nil
	}

	resp, err := da.singleflight.Do(key,
		func() (interface{}, error) {
			return da.ReadDataAccess.GetFnByID(ctx, fnID)
		})

	if err != nil {
		return nil, err
	}
	fn = resp.(*models.Fn)
	da.cache.Set(key, fn, cache.DefaultExpiration)
	return fn.(*models.Fn), nil
}
