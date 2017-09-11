package datastoreutil

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/fnproject/fn/api/models"
)

// NewValidator returns a models.Datastore which validates certain arguments before delegating to ds.
func NewValidator(ds models.Datastore) models.Datastore {
	return &validator{ds}
}

type validator struct {
	models.Datastore
}

// name will never be empty.
func (v *validator) GetApp(ctx context.Context, name string) (app *models.App, err error) {
	if name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	return v.Datastore.GetApp(ctx, name)
}

func (v *validator) GetApps(ctx context.Context, appFilter *models.AppFilter) ([]*models.App, error) {
	return v.Datastore.GetApps(ctx, appFilter)
}

// app and app.Name will never be nil/empty.
func (v *validator) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	return v.Datastore.InsertApp(ctx, app)
}

// app and app.Name will never be nil/empty.
func (v *validator) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	return v.Datastore.UpdateApp(ctx, app)
}

// name will never be empty.
func (v *validator) RemoveApp(ctx context.Context, name string) error {
	if name == "" {
		return models.ErrDatastoreEmptyAppName
	}

	return v.Datastore.RemoveApp(ctx, name)
}

// appName and routePath will never be empty.
func (v *validator) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	if routePath == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	return v.Datastore.GetRoute(ctx, appName, routePath)
}

func (v *validator) GetRoutes(ctx context.Context, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	if routeFilter != nil && routeFilter.AppName != "" {
		return v.Datastore.GetRoutesByApp(ctx, routeFilter.AppName, routeFilter)
	}

	return v.Datastore.GetRoutes(ctx, routeFilter)
}

// appName will never be empty
func (v *validator) GetRoutesByApp(ctx context.Context, appName string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	return v.Datastore.GetRoutesByApp(ctx, appName, routeFilter)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}
	if route.AppName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	if route.Path == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	return v.Datastore.InsertRoute(ctx, route)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	if newroute == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}
	if newroute.AppName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	if newroute.Path == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}
	return v.Datastore.UpdateRoute(ctx, newroute)
}

// appName and routePath will never be empty.
func (v *validator) RemoveRoute(ctx context.Context, appName, routePath string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}
	if routePath == "" {
		return models.ErrDatastoreEmptyRoutePath
	}

	return v.Datastore.RemoveRoute(ctx, appName, routePath)
}

// callID will never be empty.
func (v *validator) GetCall(ctx context.Context, appName, callID string) (*models.Call, error) {
	if callID == "" {
		return nil, models.ErrDatastoreEmptyCallID
	}
	return v.Datastore.GetCall(ctx, appName, callID)
}

func (v *validator) DeleteLog(ctx context.Context, appName, callID string) error {
	return v.Datastore.DeleteLog(ctx, appName, callID)
}

func (v *validator) BatchDeleteLogs(ctx context.Context, appName string) error {
	return v.Datastore.BatchDeleteLogs(ctx, appName)
}

func (v *validator) BatchDeleteCalls(ctx context.Context, appName string) error {
	return v.Datastore.BatchDeleteCalls(ctx, appName)
}

func (v *validator) BatchDeleteRoutes(ctx context.Context, appName string) error {
	return v.Datastore.BatchDeleteRoutes(ctx, appName)
}

// GetDatabase returns the underlying sqlx database implementation
func (v *validator) GetDatabase() *sqlx.DB {
	return v.Datastore.GetDatabase()
}
