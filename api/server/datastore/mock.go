package datastore

import "github.com/iron-io/functions/api/models"

type Mock struct{}

func (m *Mock) GetApp(app string) (*models.App, error) {
	return nil, nil
}

func (m *Mock) GetApps(appFilter *models.AppFilter) ([]*models.App, error) {
	return nil, nil
}

func (m *Mock) StoreApp(app *models.App) (*models.App, error) {
	return nil, nil
}

func (m *Mock) RemoveApp(app string) error {
	return nil
}

func (m *Mock) GetRoute(app, route string) (*models.Route, error) {
	return nil, nil
}

func (m *Mock) GetRoutes(routeFilter *models.RouteFilter) ([]*models.Route, error) {
	return nil, nil
}

func (m *Mock) StoreRoute(route *models.Route) (*models.Route, error) {
	return nil, nil
}

func (m *Mock) RemoveRoute(app, route string) error {
	return nil
}
