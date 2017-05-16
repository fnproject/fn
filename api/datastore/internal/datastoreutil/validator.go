package datastoreutil

import (
	"context"

	"gitlab.oracledx.com/odx/functions/api/models"
)

// Datastore is a copy of models.Datastore, with additional comments on parameter guarantees.
type Datastore interface {
	// name will never be empty.
	GetApp(ctx context.Context, name string) (*models.App, error)

	GetApps(ctx context.Context, appFilter *models.AppFilter) ([]*models.App, error)

	// app and app.Name will never be nil/empty.
	InsertApp(ctx context.Context, app *models.App) (*models.App, error)
	UpdateApp(ctx context.Context, app *models.App) (*models.App, error)

	// name will never be empty.
	RemoveApp(ctx context.Context, name string) error

	// appName and routePath will never be empty.
	GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error)
	RemoveRoute(ctx context.Context, appName, routePath string) error

	GetRoutes(ctx context.Context, filter *models.RouteFilter) (routes []*models.Route, err error)

	// appName will never be empty
	GetRoutesByApp(ctx context.Context, appName string, filter *models.RouteFilter) (routes []*models.Route, err error)

	// route will never be nil and route's AppName and Path will never be empty.
	InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error)
	UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error)

	// key will never be nil/empty
	Put(ctx context.Context, key, val []byte) error
	Get(ctx context.Context, key []byte) ([]byte, error)
}

// NewValidator returns a models.Datastore which validates certain arguments before delegating to ds.
func NewValidator(ds Datastore) models.Datastore {
	return &validator{ds}
}

type validator struct {
	ds Datastore
}

func (v *validator) GetApp(ctx context.Context, name string) (app *models.App, err error) {
	if name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	return v.ds.GetApp(ctx, name)
}

func (v *validator) GetApps(ctx context.Context, appFilter *models.AppFilter) ([]*models.App, error) {
	return v.ds.GetApps(ctx, appFilter)
}

func (v *validator) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	return v.ds.InsertApp(ctx, app)
}

func (v *validator) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	return v.ds.UpdateApp(ctx, app)
}

func (v *validator) RemoveApp(ctx context.Context, name string) error {
	if name == "" {
		return models.ErrDatastoreEmptyAppName
	}

	return v.ds.RemoveApp(ctx, name)
}

func (v *validator) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	if routePath == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	return v.ds.GetRoute(ctx, appName, routePath)
}

func (v *validator) GetRoutes(ctx context.Context, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	if routeFilter != nil && routeFilter.AppName != "" {
		return v.ds.GetRoutesByApp(ctx, routeFilter.AppName, routeFilter)
	}

	return v.ds.GetRoutes(ctx, routeFilter)
}

func (v *validator) GetRoutesByApp(ctx context.Context, appName string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	return v.ds.GetRoutesByApp(ctx, appName, routeFilter)
}

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

	return v.ds.InsertRoute(ctx, route)
}

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
	return v.ds.UpdateRoute(ctx, newroute)
}

func (v *validator) RemoveRoute(ctx context.Context, appName, routePath string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}
	if routePath == "" {
		return models.ErrDatastoreEmptyRoutePath
	}

	return v.ds.RemoveRoute(ctx, appName, routePath)
}

func (v *validator) Put(ctx context.Context, key, value []byte) error {
	if len(key) == 0 {
		return models.ErrDatastoreEmptyKey
	}

	return v.ds.Put(ctx, key, value)
}

func (v *validator) Get(ctx context.Context, key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, models.ErrDatastoreEmptyKey
	}
	return v.ds.Get(ctx, key)
}