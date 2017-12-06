package datastoreutil

import (
	"context"
	"io"

	"github.com/fnproject/fn/api/models"
	"github.com/jmoiron/sqlx"
	"github.com/opentracing/opentracing-go"
)

func MetricDS(ds models.Datastore) models.Datastore {
	return &metricds{ds}
}

type metricds struct {
	ds models.Datastore
}

func (m *metricds) GetApp(ctx context.Context, appName string) (*models.App, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_get_app")
	defer span.Finish()
	return m.ds.GetApp(ctx, appName)
}

func (m *metricds) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_get_apps")
	defer span.Finish()
	return m.ds.GetApps(ctx, filter)
}

func (m *metricds) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_insert_app")
	defer span.Finish()
	return m.ds.InsertApp(ctx, app)
}

func (m *metricds) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_update_app")
	defer span.Finish()
	return m.ds.UpdateApp(ctx, app)
}

func (m *metricds) RemoveApp(ctx context.Context, appName string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_remove_app")
	defer span.Finish()
	return m.ds.RemoveApp(ctx, appName)
}

func (m *metricds) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_get_route")
	defer span.Finish()
	return m.ds.GetRoute(ctx, appName, routePath)
}

func (m *metricds) GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) (routes []*models.Route, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_get_routes_by_app")
	defer span.Finish()
	return m.ds.GetRoutesByApp(ctx, appName, filter)
}

func (m *metricds) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_insert_route")
	defer span.Finish()
	return m.ds.InsertRoute(ctx, route)
}

func (m *metricds) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_update_route")
	defer span.Finish()
	return m.ds.UpdateRoute(ctx, route)
}

func (m *metricds) RemoveRoute(ctx context.Context, appName, routePath string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_remove_route")
	defer span.Finish()
	return m.ds.RemoveRoute(ctx, appName, routePath)
}

func (m *metricds) InsertCall(ctx context.Context, call *models.Call) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_insert_call")
	defer span.Finish()
	return m.ds.InsertCall(ctx, call)
}

func (m *metricds) UpdateCall(ctx context.Context, from *models.Call, to *models.Call) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_update_call")
	defer span.Finish()
	return m.ds.UpdateCall(ctx, from, to)
}

func (m *metricds) GetCall(ctx context.Context, appName, callID string) (*models.Call, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_get_call")
	defer span.Finish()
	return m.ds.GetCall(ctx, appName, callID)
}

func (m *metricds) GetCalls(ctx context.Context, filter *models.CallFilter) ([]*models.Call, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_get_calls")
	defer span.Finish()
	return m.ds.GetCalls(ctx, filter)
}

func (m *metricds) InsertLog(ctx context.Context, appName, callID string, callLog io.Reader) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_insert_log")
	defer span.Finish()
	return m.ds.InsertLog(ctx, appName, callID, callLog)
}

func (m *metricds) GetLog(ctx context.Context, appName, callID string) (io.Reader, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ds_get_log")
	defer span.Finish()
	return m.ds.GetLog(ctx, appName, callID)
}

// instant & no context ;)
func (m *metricds) GetDatabase() *sqlx.DB { return m.ds.GetDatabase() }
