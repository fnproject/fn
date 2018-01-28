package agent

import (
	"context"
	"io"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/common/singleflight"
	"github.com/fnproject/fn/api/models"
	"github.com/patrickmn/go-cache"
)

// DataAccess abstracts the datastore and message queue operations done by the
// agent, so that API nodes and runner nodes can work with the same interface
// but actually operate on the data in different ways (by direct access or by
// mediation through an API node).
type DataAccess interface {
	// GetApp abstracts querying the datastore for an app.
	GetAppByName(ctx context.Context, appName string) (*models.App, error)

	GetAppByID(ctx context.Context, appID string) (*models.App, error)

	// GetRoute abstracts querying the datastore for a route within an app.
	GetRoute(ctx context.Context, appName string, routePath string) (*models.Route, error)

	// Enqueue will add a Call to the queue (ultimately forwards to mq.Push).
	Enqueue(ctx context.Context, mCall *models.Call) error

	// Dequeue will query the queue for the next available Call that can be run
	// by this Agent, and reserve it (ultimately forwards to mq.Reserve).
	Dequeue(ctx context.Context) (*models.Call, error)

	// Start will attempt to start the provided Call within an appropriate
	// context.
	Start(ctx context.Context, mCall *models.Call) error

	// Finish will notify the system that the Call has been processed, and
	// fulfill the reservation in the queue if the call came from a queue.
	Finish(ctx context.Context, mCall *models.Call, stderr io.Reader, async bool) error
}

// CachedDataAccess wraps a DataAccess and caches the results of GetApp and GetRoute.
type CachedDataAccess struct {
	DataAccess

	cache        *cache.Cache
	singleflight singleflight.SingleFlight
}

func NewCachedDataAccess(da DataAccess) DataAccess {
	cda := &CachedDataAccess{
		DataAccess: da,
		cache:      cache.New(5*time.Second, 1*time.Minute),
	}
	return cda
}

func routeCacheKey(app, path string) string {
	return "r:" + app + "\x00" + path
}

func appIDCacheKey(appID string) string {
	return "a:" + appID
}

func appNameCacheKey(appName string) string {
	return "n:" + appName
}


func (da *CachedDataAccess) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	key := appIDCacheKey(appID)
	app, ok := da.cache.Get(key)
	if ok {
		return app.(*models.App), nil
	}

	resp, err := da.singleflight.Do(key,
		func() (interface{}, error) {
			return da.DataAccess.GetAppByID(ctx, appID)
		})

	if err != nil {
		return nil, err
	}
	app = resp.(*models.App)
	da.cache.Set(key, app, cache.DefaultExpiration)
	return app.(*models.App), nil
}

func (da *CachedDataAccess) GetAppByName(ctx context.Context, appName string) (*models.App, error) {
	return da.DataAccess.GetAppByName(ctx, appName)
}

func (da *CachedDataAccess) GetRoute(ctx context.Context, appName string, routePath string) (*models.Route, error) {
	key := routeCacheKey(appName, routePath)
	r, ok := da.cache.Get(key)
	if ok {
		return r.(*models.Route), nil
	}

	resp, err := da.singleflight.Do(key,
		func() (interface{}, error) {
			return da.DataAccess.GetRoute(ctx, appName, routePath)
		})

	if err != nil {
		return nil, err
	}
	r = resp.(*models.Route)
	da.cache.Set(key, r, cache.DefaultExpiration)
	return r.(*models.Route), nil
}

type directDataAccess struct {
	mq models.MessageQueue
	ds models.Datastore
	ls models.LogStore
}

func NewDirectDataAccess(ds models.Datastore, ls models.LogStore, mq models.MessageQueue) DataAccess {
	da := &directDataAccess{
		mq: mq,
		ds: ds,
		ls: ls,
	}
	return da
}

func (da *directDataAccess) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	return da.ds.GetAppByID(ctx, appID)
}

func (da *directDataAccess) GetAppByName(ctx context.Context, appName string) (*models.App, error) {
	return da.ds.GetAppByName(ctx, appName)
}

// to make it work for both hybrid and full mode "app" parameter would be app name,
// for direct access we need app ID hence we need to get app object by app name
func (da *directDataAccess) GetRoute(ctx context.Context, app string, routePath string) (*models.Route, error) {
	a, err := da.ds.GetAppByName(ctx, app)
	if err != nil {
		return nil, err
	}
	return da.ds.GetRoute(ctx, a.ID, routePath)
}

func (da *directDataAccess) Enqueue(ctx context.Context, mCall *models.Call) error {
	_, err := da.mq.Push(ctx, mCall)
	return err
	// TODO: Insert a call in the datastore with the 'queued' state
}

func (da *directDataAccess) Dequeue(ctx context.Context) (*models.Call, error) {
	return da.mq.Reserve(ctx)
}

func (da *directDataAccess) Start(ctx context.Context, mCall *models.Call) error {
	// TODO Access datastore and try a Compare-And-Swap to set the call to
	// 'running'. If it fails, delete the message from the MQ and return an
	// error. If it is successful, don't do anything - the message will be
	// removed when the call Finish'es.

	// At the moment we don't have the queued/running/finished mechanics so we
	// remove the message here.
	return da.mq.Delete(ctx, mCall)
}

func (da *directDataAccess) Finish(ctx context.Context, mCall *models.Call, stderr io.Reader, async bool) error {
	// this means that we could potentially store an error / timeout status for a
	// call that ran successfully [by a user's perspective]
	// TODO: this should be update, really
	if err := da.ds.InsertCall(ctx, mCall); err != nil {
		common.Logger(ctx).WithError(err).Error("error inserting call into datastore")
		// note: Not returning err here since the job could have already finished successfully.
	}

	if err := da.ls.InsertLog(ctx, mCall.AppID, mCall.ID, stderr); err != nil {
		common.Logger(ctx).WithError(err).Error("error uploading log")
		// note: Not returning err here since the job could have already finished successfully.
	}

	if async {
		// XXX (reed): delete MQ message, eventually
		// YYY (hhexo): yes, once we have the queued/running/finished mechanics
		// return da.mq.Delete(ctx, mCall)
	}
	return nil
}
