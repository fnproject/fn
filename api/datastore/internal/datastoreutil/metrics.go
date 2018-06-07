package datastoreutil

import (
	"context"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/models"
	"github.com/jmoiron/sqlx"
)

func MetricDS(ds models.Datastore) models.Datastore {
	return &metricds{ds}
}

type metricds struct {
	ds models.Datastore
}

func (m *metricds) GetAppID(ctx context.Context, appName string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_app_id")
	defer span.End()
	return m.ds.GetAppID(ctx, appName)
}

func (m *metricds) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_app_by_id")
	defer span.End()
	return m.ds.GetAppByID(ctx, appID)
}

func (m *metricds) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_apps")
	defer span.End()
	return m.ds.GetApps(ctx, filter)
}

func (m *metricds) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	ctx, span := trace.StartSpan(ctx, "ds_insert_app")
	defer span.End()
	return m.ds.InsertApp(ctx, app)
}

func (m *metricds) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	ctx, span := trace.StartSpan(ctx, "ds_update_app")
	defer span.End()
	return m.ds.UpdateApp(ctx, app)
}

func (m *metricds) RemoveApp(ctx context.Context, appID string) error {
	ctx, span := trace.StartSpan(ctx, "ds_remove_app")
	defer span.End()
	return m.ds.RemoveApp(ctx, appID)
}

func (m *metricds) GetRoute(ctx context.Context, appID, routePath string) (*models.Route, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_route")
	defer span.End()
	return m.ds.GetRoute(ctx, appID, routePath)
}

func (m *metricds) GetRoutesByApp(ctx context.Context, appID string, filter *models.RouteFilter) (routes []*models.Route, err error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_routes_by_app")
	defer span.End()
	return m.ds.GetRoutesByApp(ctx, appID, filter)
}

func (m *metricds) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	ctx, span := trace.StartSpan(ctx, "ds_insert_route")
	defer span.End()
	return m.ds.InsertRoute(ctx, route)
}

func (m *metricds) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	ctx, span := trace.StartSpan(ctx, "ds_update_route")
	defer span.End()
	return m.ds.UpdateRoute(ctx, route)
}

func (m *metricds) RemoveRoute(ctx context.Context, appID string, routePath string) error {
	ctx, span := trace.StartSpan(ctx, "ds_remove_route")
	defer span.End()
	return m.ds.RemoveRoute(ctx, appID, routePath)
}

func (m *metricds) PutFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "ds_put_func")
	defer span.End()
	return m.ds.PutFn(ctx, fn)
}

func (m *metricds) GetFns(ctx context.Context, filter *models.FnFilter) ([]*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_funcs")
	defer span.End()
	return m.ds.GetFns(ctx, filter)
}

func (m *metricds) GetFn(ctx context.Context, funcName string) (*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_func")
	defer span.End()
	return m.ds.GetFn(ctx, funcName)
}

func (m *metricds) RemoveFn(ctx context.Context, funcName string) error {
	ctx, span := trace.StartSpan(ctx, "ds_remove_func")
	defer span.End()
	return m.ds.RemoveFn(ctx, funcName)
}

// instant & no context ;)
func (m *metricds) GetDatabase() *sqlx.DB { return m.ds.GetDatabase() }

// Close calls Close on the underlying Datastore
func (m *metricds) Close() error {
	return m.ds.Close()
}
