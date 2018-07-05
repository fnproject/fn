package datastoreutil

import (
	"context"

	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/models"
)

func MetricDS(ds models.Datastore) models.Datastore {
	return &metricds{ds}
}

type metricds struct {
	ds models.Datastore
}

func (m *metricds) GetTriggerBySource(ctx context.Context, appId string, triggerType, source string) (*models.Trigger, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_trigger_by_source")
	defer span.End()
	return m.ds.GetTriggerBySource(ctx, appId, triggerType, source)
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

func (m *metricds) GetApps(ctx context.Context, filter *models.AppFilter) (*models.AppList, error) {
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

func (m *metricds) InsertTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {
	ctx, span := trace.StartSpan(ctx, "ds_insert_trigger")
	defer span.End()
	return m.ds.InsertTrigger(ctx, trigger)

}

func (m *metricds) UpdateTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {
	ctx, span := trace.StartSpan(ctx, "ds_update_trigger")
	defer span.End()
	return m.ds.UpdateTrigger(ctx, trigger)
}

func (m *metricds) RemoveTrigger(ctx context.Context, triggerID string) error {
	ctx, span := trace.StartSpan(ctx, "ds_remove_trigger")
	defer span.End()
	return m.ds.RemoveTrigger(ctx, triggerID)
}

func (m *metricds) GetTriggerByID(ctx context.Context, triggerID string) (*models.Trigger, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_trigger_by_id")
	defer span.End()
	return m.ds.GetTriggerByID(ctx, triggerID)
}

func (m *metricds) GetTriggers(ctx context.Context, filter *models.TriggerFilter) (*models.TriggerList, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_triggers")
	defer span.End()
	return m.ds.GetTriggers(ctx, filter)
}

func (m *metricds) InsertFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "ds_insert_func")
	defer span.End()
	return m.ds.InsertFn(ctx, fn)
}

func (m *metricds) UpdateFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "ds_insert_func")
	defer span.End()
	return m.ds.UpdateFn(ctx, fn)
}

func (m *metricds) GetFns(ctx context.Context, filter *models.FnFilter) (*models.FnList, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_funcs")
	defer span.End()
	return m.ds.GetFns(ctx, filter)
}

func (m *metricds) GetFnByID(ctx context.Context, fnID string) (*models.Fn, error) {
	ctx, span := trace.StartSpan(ctx, "ds_get_func")
	defer span.End()
	return m.ds.GetFnByID(ctx, fnID)
}

func (m *metricds) RemoveFn(ctx context.Context, fnID string) error {
	ctx, span := trace.StartSpan(ctx, "ds_remove_func")
	defer span.End()
	return m.ds.RemoveFn(ctx, fnID)
}

// Close calls Close on the underlying Datastore
func (m *metricds) Close() error {
	return m.ds.Close()
}
