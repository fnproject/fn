package hybrid

import (
	"context"
	"errors"
	"io"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/models"
	"go.opencensus.io/trace"
)

// nopDataStore implements agent.DataAccess
type nopDataStore struct{}

func (cl *nopDataStore) GetTriggerBySource(ctx context.Context, appId string, triggerType, source string) (*models.Trigger, error) {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_get_trigger_by_source")
	defer span.End()
	return nil, errors.New("should not call GetTriggerBySource on a NOP data store")
}

func (cl *nopDataStore) GetFnByID(ctx context.Context, fnId string) (*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_get_fn_by_id")
	defer span.End()
	return nil, errors.New("should not call GetFnByID on a NOP data store")
}

func NewNopDataStore() (agent.DataAccess, error) {
	return &nopDataStore{}, nil
}

func (cl *nopDataStore) GetAppID(ctx context.Context, appName string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_get_app_id")
	defer span.End()
	return "", errors.New("should not call GetAppID on a NOP data store")
}

func (cl *nopDataStore) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_get_app_by_id")
	defer span.End()
	return nil, errors.New("should not call GetAppByID on a NOP data store")
}

func (cl *nopDataStore) Enqueue(ctx context.Context, c *models.Call) error {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_enqueue")
	defer span.End()
	return errors.New("Should not call Enqueue on a NOP data store")
}

func (cl *nopDataStore) Dequeue(ctx context.Context) (*models.Call, error) {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_dequeue")
	defer span.End()
	return nil, errors.New("Should not call Dequeue on a NOP data store")
}

func (cl *nopDataStore) Start(ctx context.Context, c *models.Call) error {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_start")
	defer span.End()
	return nil // It's ok to call this method, and it does no operations
}

func (cl *nopDataStore) Finish(ctx context.Context, c *models.Call, r io.Reader, async bool) error {
	ctx, span := trace.StartSpan(ctx, "nop_datastore_end")
	defer span.End()
	return nil // It's ok to call this method, and it does no operations
}

func (cl *nopDataStore) Close() error {
	return nil
}
