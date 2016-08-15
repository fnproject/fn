package datastore

import "github.com/iron-io/functions/api/models"

type Mock struct {
	FakeApp    *models.App
	FakeApps   []*models.App
	FakeRoute  *models.Route
	FakeRoutes []*models.Route
}

func (m *Mock) GetApp(app string) (*models.App, error) {
	return m.FakeApp, nil
}

func (m *Mock) GetApps(appFilter *models.AppFilter) ([]*models.App, error) {
	return m.FakeApps, nil
}

func (m *Mock) StoreApp(app *models.App) (*models.App, error) {
	return m.FakeApp, nil
}

func (m *Mock) RemoveApp(app string) error {
	return nil
}

func (m *Mock) GetRoute(app, route string) (*models.Route, error) {
	return m.FakeRoute, nil
}

func (m *Mock) GetRoutes(routeFilter *models.RouteFilter) ([]*models.Route, error) {
	return m.FakeRoutes, nil
}

func (m *Mock) GetRoutesByApp(appName string, routeFilter *models.RouteFilter) ([]*models.Route, error) {
	return m.FakeRoutes, nil
}

func (m *Mock) StoreRoute(route *models.Route) (*models.Route, error) {
	return m.FakeRoute, nil
}

func (m *Mock) RemoveRoute(app, route string) error {
	return nil
}

func (m *Mock) Put(key, value []byte) error {
	return nil
}

func (m *Mock) Get(key []byte) ([]byte, error) {
	return []byte{}, nil
}
