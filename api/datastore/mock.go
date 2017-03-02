package datastore

import (
	"github.com/iron-io/functions/api/models"

	"context"
)

type Mock struct {
	Apps   []*models.App
	Routes []*models.Route
	data map[string][]byte
}

func NewMock(apps []*models.App, routes []*models.Route) *Mock {
	if apps == nil {
		apps = []*models.App{}
	}
	if routes == nil {
		routes = []*models.Route{}
	}
	return &Mock{apps, routes, make(map[string][]byte)}
}

func (m *Mock) GetApp(ctx context.Context, appName string) (app *models.App, err error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	for _, a := range m.Apps {
		if a.Name == appName {
			return a, nil
		}
	}

	return nil, models.ErrAppsNotFound
}

func (m *Mock) GetApps(ctx context.Context, appFilter *models.AppFilter) ([]*models.App, error) {
	return m.Apps, nil
}

func (m *Mock) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.Name == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if a, _ := m.GetApp(ctx, app.Name); a != nil {
		return nil, models.ErrAppsAlreadyExists
	}
	m.Apps = append(m.Apps, app)
	return app, nil
}

func (m *Mock) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	a, err := m.GetApp(ctx, app.Name)
	if err != nil {
		return nil, err
	}
	a.UpdateConfig(app.Config)

	return a.Clone(), nil
}

func (m *Mock) RemoveApp(ctx context.Context, appName string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}
	for i, a := range m.Apps {
		if a.Name == appName {
			m.Apps = append(m.Apps[:i], m.Apps[i+1:]...)
			return nil
		}
	}
	return models.ErrAppsNotFound
}

func (m *Mock) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	if appName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}
	if routePath == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}
	for _, r := range m.Routes {
		if r.AppName == appName && r.Path == routePath {
			return r, nil
		}
	}
	return nil, models.ErrRoutesNotFound
}

func (m *Mock) GetRoutes(ctx context.Context, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	for _, r := range m.Routes {
		routes = append(routes, r)
	}
	return
}

func (m *Mock) GetRoutesByApp(ctx context.Context, appName string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	for _, r := range m.Routes {
		if r.AppName == appName && (routeFilter.Path == "" || r.Path == routeFilter.Path) && (routeFilter.AppName == "" || r.AppName == routeFilter.AppName) {
			routes = append(routes, r)
		}
	}
	return
}

func (m *Mock) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	if route.AppName == "" {
		return nil, models.ErrDatastoreEmptyAppName
	}

	if route.Path == "" {
		return nil, models.ErrDatastoreEmptyRoutePath
	}

	if _, err := m.GetApp(ctx, route.AppName); err != nil {
		return nil, err
	}

	if r, _ := m.GetRoute(ctx, route.AppName, route.Path); r != nil {
		return nil, models.ErrRoutesAlreadyExists
	}
	m.Routes = append(m.Routes, route)
	return route, nil
}

func (m *Mock) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	r, err := m.GetRoute(ctx, route.AppName, route.Path)
	if err != nil {
		return nil, err
	}
	r.Update(route)
	return r.Clone(), nil
}

func (m *Mock) RemoveRoute(ctx context.Context, appName, routePath string) error {
	if appName == "" {
		return models.ErrDatastoreEmptyAppName
	}
	if routePath == "" {
		return models.ErrDatastoreEmptyRoutePath
	}
	for i, r := range m.Routes {
		if r.AppName == appName && r.Path == routePath {
			m.Routes = append(m.Routes[:i], m.Routes[i+1:]...)
			return nil
		}
	}
	return models.ErrRoutesNotFound
}

func (m *Mock) Put(ctx context.Context, key, value []byte) error {
	if len(key) == 0 {
		return models.ErrDatastoreEmptyKey
	}
	if len(value) == 0 {
		delete(m.data, string(key))
	} else {
		m.data[string(key)] = value
	}
	return nil
}

func (m *Mock) Get(ctx context.Context, key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, models.ErrDatastoreEmptyKey
	}
	return m.data[string(key)], nil
}
