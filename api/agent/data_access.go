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

//ReadDataAccess represents read operations required to operate a load balancer node
type ReadDataAccess interface {
	GetAppID(ctx context.Context, appName string) (string, error)
	// GetAppByID abstracts querying the datastore for an app.
	GetAppByID(ctx context.Context, appID string) (*models.App, error)
	GetTriggerBySource(ctx context.Context, appId string, triggerType, source string) (*models.Trigger, error)
	GetFnByID(ctx context.Context, fnId string) (*models.Fn, error)
	// GetRoute abstracts querying the datastore for a route within an app.
	GetRoute(ctx context.Context, appID string, routePath string) (*models.Route, error)
}

//DequeueDataAccess abstracts an underlying dequeue for async runners
type DequeueDataAccess interface {
	// Dequeue will query the queue for the next available Call that can be run
	// by this Agent, and reserve it (ultimately forwards to mq.Reserve).
	Dequeue(ctx context.Context) (*models.Call, error)
}

//EnqueueDataAccess abstracts an underying enqueue for async queueing
type EnqueueDataAccess interface {
	// Enqueue will add a Call to the queue (ultimately forwards to mq.Push).
	Enqueue(ctx context.Context, mCall *models.Call) error
}

// CallHandler consumes the start and finish events for a call
// This is effectively a callback that is allowed to read the logs -
// TODO Deprecate this - this could be a CallListener except it also consumes logs
type CallHandler interface {
	// Start will attempt to start the provided Call within an appropriate
	// context.
	Start(ctx context.Context, mCall *models.Call) error

	// Finish will notify the system that the Call has been processed, and
	// fulfill the reservation in the queue if the call came from a queue.
	Finish(ctx context.Context, mCall *models.Call, stderr io.Reader, async bool) error
}

// DataAccess is currently
type DataAccess interface {
	ReadDataAccess
	DequeueDataAccess
	CallHandler
}

// CachedDataAccess wraps a DataAccess and caches the results of GetApp and GetRoute.
type cachedDataAccess struct {
	ReadDataAccess

	cache        *cache.Cache
	singleflight singleflight.SingleFlight
}

func NewCachedDataAccess(da ReadDataAccess) ReadDataAccess {
	cda := &cachedDataAccess{
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

func (da *cachedDataAccess) GetRoute(ctx context.Context, appID string, routePath string) (*models.Route, error) {
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

type directDataAccess struct {
	mq models.MessageQueue
	ls models.LogStore
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

	if err := da.ls.InsertLog(ctx, mCall.AppID, mCall.FnID, mCall.ID, stderr); err != nil {
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

type noAsyncEnqueueAccess struct{}

func (noAsyncEnqueueAccess) Enqueue(ctx context.Context, mCall *models.Call) error {
	return models.ErrAsyncUnsupported
}

//NewUnsupportedEnqueueAccess is a backstop that errors when you try to enqueue an async operation on a server that doesn't support async
func NewUnsupportedAsyncEnqueueAccess() EnqueueDataAccess {
	return &noAsyncEnqueueAccess{}
}
