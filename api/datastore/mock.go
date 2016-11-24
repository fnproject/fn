package datastore

import (
	"github.com/iron-io/functions/api/models"

	"context"
)

type Mock struct {
	Apps   []*models.App
	Routes []*models.Route
}

func NewMock(apps []*models.App, routes []*models.Route) *Mock {
	if apps == nil {
		apps = []*models.App{}
	}
	if routes == nil {
		routes = []*models.Route{}
	}
	return &Mock{apps, routes}
}

func (m *Mock) GetApp(ctx context.Context, appName string) (app *models.App, err error) {
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
	if app.Config != nil {
		if a.Config == nil {
			a.Config = map[string]string{}
		}
		for k, v := range app.Config {
			a.Config[k] = v
		}
	}
	return a, nil
}

func (m *Mock) RemoveApp(ctx context.Context, appName string) error {
	for i, a := range m.Apps {
		if a.Name == appName {
			m.Apps = append(m.Apps[:i], m.Apps[i+1:]...)
			return nil
		}
	}
	return models.ErrAppsNotFound
}

func (m *Mock) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
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
	if r, _ := m.GetRoute(ctx, route.AppName, route.Path); r != nil {
		return nil, models.ErrAppsAlreadyExists
	}
	m.Routes = append(m.Routes, route)
	return route, nil
}

func (m *Mock) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	r, err := m.GetRoute(ctx, route.AppName, route.Path)
	if err != nil {
		return nil, err
	}
	if route.Config != nil {
		if route.Config == nil {
			r.Config = map[string]string{}
		}
		for k, v := range route.Config {
			r.Config[k] = v
		}
	}
	return r, nil
}

func (m *Mock) RemoveRoute(ctx context.Context, appName, routePath string) error {
	for i, r := range m.Routes {
		if r.AppName == appName && r.Path == routePath {
			m.Routes = append(m.Routes[:i], m.Routes[i+1:]...)
			return nil
		}
	}
	return models.ErrRoutesNotFound
}

func (m *Mock) Put(ctx context.Context, key, value []byte) error {
	// TODO: improve this mock method
	return nil
}

func (m *Mock) Get(ctx context.Context, key []byte) ([]byte, error) {
	// TODO: improve this mock method
	return []byte{}, nil
}
