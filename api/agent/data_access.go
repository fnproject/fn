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

type ReadDataAccess interface {
	GetAppID(ctx context.Context, appName string) (string, error)
	// GetAppByID abstracts querying the datastore for an app.
	GetAppByID(ctx context.Context, appID string) (*models.App, error)
	GetTriggerBySource(ctx context.Context, appId string, triggerType, source string) (*models.Trigger, error)
	GetFnByID(ctx context.Context, fnId string) (*models.Fn, error)
	// GetRoute abstracts querying the datastore for a route within an app.
	GetRoute(ctx context.Context, appID string, routePath string) (*models.Route, error)
}

type DequeueDataAccess interface {
	// Dequeue will query the queue for the next available Call that can be run
	// by this Agent, and reserve it (ultimately forwards to mq.Reserve).
	Dequeue(ctx context.Context) (*models.Call, error)
}

type EnqueueDataAccess interface {
	// Enqueue will add a Call to the queue (ultimately forwards to mq.Push).
	Enqueue(ctx context.Context, mCall *models.Call) error
}

// CallHandler consumes the start and finish events for a call
type CallHandler interface {
	io.Closer
	// Start will attempt to start the provided Call within an appropriate
	// context.
	Start(ctx context.Context, mCall *models.Call) error

	// Finish will notify the system that the Call has been processed, and
	// fulfill the reservation in the queue if the call came from a queue.
	Finish(ctx context.Context, mCall *models.Call, stderr io.Reader, async bool) error
}

// DataAccess abstracts the datastore and message queue operations done by the
// agent, so that API nodes and runner nodes can work with the same interface
// but actually operate on the data in different ways (by direct access or by
// mediation through an API node).
type DataAccess interface {
	ReadDataAccess
	DequeueDataAccess
	CallHandler

	// Close will wait for any pending operations to complete and
	// shuts down connections to the underlying datastore/queue resources.
	// Close is not safe to be called from multiple threads.
	//io.Closer
}

// CachedDataAccess wraps a DataAccess and caches the results of GetApp and GetRoute.
type CachedDataAccess struct {
	ReadDataAccess

	cache        *cache.Cache
	singleflight singleflight.SingleFlight
}

func NewCachedDataAccess(da ReadDataAccess) ReadDataAccess {
	cda := &CachedDataAccess{
		ReadDataAccess: da,
		cache:          cache.New(5*time.Second, 1*time.Minute),
	}
	return cda
}

func routeCacheKey(app, path string) string {
	return "r:" + app + "\x00" + path
}

func appIDCacheKey(appID string) string {
	return "a:" + appID
}

func (da *CachedDataAccess) GetAppID(ctx context.Context, appName string) (string, error) {
	return da.ReadDataAccess.GetAppID(ctx, appName)
}

func (da *CachedDataAccess) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
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

func (da *CachedDataAccess) GetRoute(ctx context.Context, appID string, routePath string) (*models.Route, error) {
	key := routeCacheKey(appID, routePath)
	r, ok := da.cache.Get(key)
	if ok {
		return r.(*models.Route), nil
	}

	resp, err := da.singleflight.Do(key,
		func() (interface{}, error) {
			return da.ReadDataAccess.GetRoute(ctx, appID, routePath)
		})

	if err != nil {
		return nil, err
	}
	r = resp.(*models.Route)
	da.cache.Set(key, r, cache.DefaultExpiration)
	return r.(*models.Route), nil
}

//// Close invokes close on the underlying DataAccess
//func (cda *CachedDataAccess) Close() error {
//	return cda.ReadDataAccess.Close()
//}

type directDataAccess struct {
	mq models.MessageQueue
	ls models.LogStore
}

type directReadAccess struct {
	models.Datastore
}

func NewDirectReadAccess(ds models.Datastore) ReadDataAccess {
	return &directReadAccess{
		Datastore: ds,
	}
}

type directDequeue struct {
	mq models.MessageQueue
}

func (ddq *directDequeue) Dequeue(ctx context.Context) (*models.Call, error) {
	return ddq.mq.Reserve(ctx)
}

func NewDirectDequeueAccess(mq models.MessageQueue) DequeueDataAccess {
	return &directDequeue{
		mq: mq,
	}
}

type directEnequeue struct {
	mq models.MessageQueue
}

func NewDirectEnqueueAccess(mq models.MessageQueue) EnqueueDataAccess {
	return &directEnequeue{
		mq: mq,
	}
}

func (da *directEnequeue) Enqueue(ctx context.Context, mCall *models.Call) error {
	_, err := da.mq.Push(ctx, mCall)
	return err
	// TODO: Insert a call in the datastore with the 'queued' state
}

func NewDirectCallDataAccess(ls models.LogStore, mq models.MessageQueue) CallHandler {
	da := &directDataAccess{
		mq: mq,
		ls: ls,
	}
	return da
}

func (da *directDataAccess) Enqueue(ctx context.Context, mCall *models.Call) error {
	_, err := da.mq.Push(ctx, mCall)
	return err
	// TODO: Insert a call in the datastore with the 'queued' state
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
	if err := da.ls.InsertCall(ctx, mCall); err != nil {
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
		// return cda.mq.Delete(ctx, mCall)
	}
	return nil
}

// Close calls close on the underlying Datastore and MessageQueue. If the Logstore
// and Datastore are different, it will call Close on the Logstore as well.
func (da *directDataAccess) Close() error {
	// TRIGGERWIP: Make sure DS is still correctly closed in server
	//err := handler.ds.Close()
	//if ls, ok := handler.ds.(models.LogStore); ok && ls != handler.ls {
	//	if daErr := handler.ls.Close(); daErr != nil {
	//		err = daErr
	//	}
	//}
	return da.mq.Close()

}
