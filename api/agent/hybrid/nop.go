package hybrid

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/models"
	"go.opencensus.io/trace"
)

// nopDataStore implements agent.DataAccess
type nopDataStore struct{}

func NewNopDataStore() (agent.DataAccess, error) {
	return &nopDataStore{}, nil
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

func (cl *nopDataStore) GetApp(req *http.Request, appName string) (*models.App, error) {
	_, span := trace.StartSpan(req.Context(), "nop_datastore_get_app")
	defer span.End()
	return nil, errors.New("Should not call GetApp on a NOP data store")
}

func (cl *nopDataStore) GetRoute(req *http.Request, appName, route string) (*models.Route, error) {
	_, span := trace.StartSpan(req.Context(), "nop_datastore_get_route")
	defer span.End()
	return nil, errors.New("Should not call GetRoute on a NOP data store")
}
