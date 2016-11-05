// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datastore

import "github.com/iron-io/functions/api/models"

type Mock struct {
	FakeApp    *models.App
	FakeApps   []*models.App
	FakeRoute  *models.Route
	FakeRoutes []*models.Route
}

func (m *Mock) GetApp(appName string) (*models.App, error) {
	app := m.FakeApp
	if app == nil && m.FakeApps != nil {
		for _, a := range m.FakeApps {
			if a.Name == appName {
				app = a
			}
		}
	}

	return app, nil
}

func (m *Mock) GetApps(appFilter *models.AppFilter) ([]*models.App, error) {
	// TODO: improve this mock method
	return m.FakeApps, nil
}

func (m *Mock) StoreApp(app *models.App) (*models.App, error) {
	// TODO: improve this mock method
	return m.FakeApp, nil
}

func (m *Mock) RemoveApp(appName string) error {
	// TODO: improve this mock method
	return nil
}

func (m *Mock) GetRoute(appName, routePath string) (*models.Route, error) {
	route := m.FakeRoute
	if route == nil && m.FakeRoutes != nil {
		for _, r := range m.FakeRoutes {
			if r.AppName == appName && r.Path == routePath {
				route = r
			}
		}
	}

	return route, nil
}

func (m *Mock) GetRoutes(routeFilter *models.RouteFilter) ([]*models.Route, error) {
	// TODO: improve this mock method
	return m.FakeRoutes, nil
}

func (m *Mock) GetRoutesByApp(appName string, routeFilter *models.RouteFilter) ([]*models.Route, error) {
	var routes []*models.Route
	route := m.FakeRoute
	if route == nil && m.FakeRoutes != nil {
		for _, r := range m.FakeRoutes {
			if r.AppName == appName && r.Path == routeFilter.Path && r.AppName == routeFilter.AppName {
				routes = append(routes, r)
			}
		}
	}

	return routes, nil
}

func (m *Mock) StoreRoute(route *models.Route) (*models.Route, error) {
	// TODO: improve this mock method
	return m.FakeRoute, nil
}

func (m *Mock) RemoveRoute(appName, routePath string) error {
	// TODO: improve this mock method
	return nil
}

func (m *Mock) Put(key, value []byte) error {
	// TODO: improve this mock method
	return nil
}

func (m *Mock) Get(key []byte) ([]byte, error) {
	// TODO: improve this mock method
	return []byte{}, nil
}
