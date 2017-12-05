package agent

import (
	"context"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"io"
)

// DataAccess abstracts the datastore and message queue operations done by the
// agent, so that API nodes and runner nodes can work with the same interface
// but actually operate on the data in different ways (by direct access or by
// mediation through an API node).
type DataAccess interface {
	// GetApp abstracts querying the datastore for an app.
	GetApp(ctx context.Context, appName string) (*models.App, error)

	// GetRoute abstracts querying the datastore for a route within an app.
	GetRoute(ctx context.Context, appName string, routePath string) (*models.Route, error)

	// Enqueue will add a Call to the queue (ultimately forwards to mq.Push).
	Enqueue(ctx context.Context, mCall *models.Call) (*models.Call, error)

	// Dequeue will query the queue for the next available Call that can be run
	// by this Agent, and reserve it (ultimately forwards to mq.Reserve).
	Dequeue(ctx context.Context) (*models.Call, error)

	// Start will attempt to start the provided Call within an appropriate
	// context.
	Start(ctx context.Context, mCall *models.Call) error

	// Finish will notify the system that the Call has been processed, and
	// fulfill the reservation in the queue if the call came from a queue.
	Finish(ctx context.Context, mCall *models.Call, stderr io.ReadWriteCloser, async bool) error
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

func (da *directDataAccess) GetApp(ctx context.Context, appName string) (*models.App, error) {
	return da.ds.GetApp(ctx, appName)
}

func (da *directDataAccess) GetRoute(ctx context.Context, appName string, routePath string) (*models.Route, error) {
	return da.ds.GetRoute(ctx, appName, routePath)
}

func (da *directDataAccess) Enqueue(ctx context.Context, mCall *models.Call) (*models.Call, error) {
	return da.mq.Push(ctx, mCall)
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

func (da *directDataAccess) Finish(ctx context.Context, mCall *models.Call, stderr io.ReadWriteCloser, async bool) error {
	// this means that we could potentially store an error / timeout status for a
	// call that ran successfully [by a user's perspective]
	// TODO: this should be update, really
	if err := da.ds.InsertCall(ctx, mCall); err != nil {
		common.Logger(ctx).WithError(err).Error("error inserting call into datastore")
		// note: Not returning err here since the job could have already finished successfully.
	}

	if err := da.ls.InsertLog(ctx, mCall.AppName, mCall.ID, stderr); err != nil {
		common.Logger(ctx).WithError(err).Error("error uploading log")
		// note: Not returning err here since the job could have already finished successfully.
	}
	// NOTE call this after InsertLog or the buffer will get reset
	stderr.Close()

	if async {
		// XXX (reed): delete MQ message, eventually
		// YYY (hhexo): yes, once we have the queued/running/finished mechanics
		// return da.mq.Delete(ctx, mCall)
	}
	return nil
}
