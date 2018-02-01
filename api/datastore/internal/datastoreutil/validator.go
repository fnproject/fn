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
		return nil, models.ErrAppsMissingName
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

	app.SetDefaults()
	if err := app.Validate(); err != nil {
		return nil, err
	}

	return v.Datastore.InsertApp(ctx, app)
}

// app and app.Name will never be nil/empty.
func (v *validator) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return nil, models.ErrAppsMissingName
	}
	return v.Datastore.UpdateApp(ctx, app)
}

// name will never be empty.
func (v *validator) RemoveApp(ctx context.Context, name string) error {
	if name == "" {
		return models.ErrAppsMissingName
	}

	return v.Datastore.RemoveApp(ctx, name)
}

// appName and routePath will never be empty.
func (v *validator) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	if appName == "" {
		return nil, models.ErrAppsMissingName
	}
	if routePath == "" {
		return nil, models.ErrRoutesMissingPath
	}

	return v.Datastore.GetRoute(ctx, appName, routePath)
}

// appName will never be empty
func (v *validator) GetRoutesByApp(ctx context.Context, appName string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	if appName == "" {
		return nil, models.ErrAppsMissingName
	}
	return v.Datastore.GetRoutesByApp(ctx, appName, routeFilter)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	route.SetDefaults()
	if err := route.Validate(); err != nil {
		return nil, err
	}

	return v.Datastore.InsertRoute(ctx, route)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	if newroute == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}
	if newroute.AppName == "" {
		return nil, models.ErrAppsMissingName
	}
	if newroute.Path == "" {
		return nil, models.ErrRoutesMissingPath
	}
	return v.Datastore.UpdateRoute(ctx, newroute)
}

// appName and routePath will never be empty.
func (v *validator) RemoveRoute(ctx context.Context, appName, routePath string) error {
	if appName == "" {
		return models.ErrAppsMissingName
	}
	if routePath == "" {
		return models.ErrRoutesMissingPath
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

// GetDatabase returns the underlying sqlx database implementation
func (v *validator) GetDatabase() *sqlx.DB {
	return v.Datastore.GetDatabase()
}
