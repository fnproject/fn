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

func (v *validator) GetAppID(ctx context.Context, appName string) (string, error) {
	if appName == "" {
		return "", models.ErrAppsMissingName
	}
	return v.Datastore.GetAppID(ctx, appName)
}

func (v *validator) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	if appID == "" {
		return nil, models.ErrDatastoreEmptyAppID
	}

	return v.Datastore.GetAppByID(ctx, appID)
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
	if app.ID == "" {
		return nil, models.ErrDatastoreEmptyAppID
	}

	return v.Datastore.UpdateApp(ctx, app)
}

// name will never be empty.
func (v *validator) RemoveApp(ctx context.Context, appID string) error {
	if appID == "" {
		return models.ErrDatastoreEmptyAppID
	}

	return v.Datastore.RemoveApp(ctx, appID)
}

// appName and routePath will never be empty.
func (v *validator) GetRoute(ctx context.Context, appID, routePath string) (*models.Route, error) {
	if appID == "" {
		return nil, models.ErrDatastoreEmptyAppID
	}
	if routePath == "" {
		return nil, models.ErrRoutesMissingPath
	}

	return v.Datastore.GetRoute(ctx, appID, routePath)
}

// appName will never be empty
func (v *validator) GetRoutesByApp(ctx context.Context, appID string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	if appID == "" {
		return nil, models.ErrDatastoreEmptyAppID
	}

	return v.Datastore.GetRoutesByApp(ctx, appID, routeFilter)
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
	if newroute.AppID == "" {
		return nil, models.ErrRoutesMissingAppID
	}
	if newroute.Path == "" {
		return nil, models.ErrRoutesMissingPath
	}
	return v.Datastore.UpdateRoute(ctx, newroute)
}

// appName and routePath will never be empty.
func (v *validator) RemoveRoute(ctx context.Context, appID string, routePath string) error {
	if appID == "" {
		return models.ErrDatastoreEmptyAppID
	}
	if routePath == "" {
		return models.ErrRoutesMissingPath
	}

	return v.Datastore.RemoveRoute(ctx, appID, routePath)
}

func (v *validator) PutFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	if fn == nil {
		return nil, models.ErrDatastoreEmptyFn
	}
	if fn.Name == "" {
		return nil, models.ErrDatastoreEmptyFnName
	}
	return v.Datastore.PutFn(ctx, fn)
}

func (v *validator) GetFn(ctx context.Context, funcName string) (*models.Fn, error) {
	if funcName == "" {
		return nil, models.ErrDatastoreEmptyFnName
	}
	return v.Datastore.GetFn(ctx, funcName)
}

func (v *validator) RemoveFn(ctx context.Context, funcName string) error {
	if funcName == "" {
		return models.ErrDatastoreEmptyFnName
	}
	return v.Datastore.RemoveFn(ctx, funcName)
}

// GetDatabase returns the underlying sqlx database implementation
func (v *validator) GetDatabase() *sqlx.DB {
	return v.Datastore.GetDatabase()
}
