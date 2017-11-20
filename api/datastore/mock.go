package datastore

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/go-openapi/strfmt"
	"github.com/jmoiron/sqlx"
)

type mock struct {
	Apps   []*models.App
	Routes []*models.Route
	Calls  []*models.Call
	data   map[string][]byte

	models.LogStore
}

func NewMock() models.Datastore {
	return NewMockInit(nil, nil, nil)
}

func NewMockInit(apps []*models.App, routes []*models.Route, calls []*models.Call) models.Datastore {
	return datastoreutil.NewValidator(&mock{apps, routes, calls, make(map[string][]byte), logs.NewMock()})
}

func (m *mock) GetApp(ctx context.Context, appName string) (app *models.App, err error) {
	for _, a := range m.Apps {
		if a.Name == appName {
			return a, nil
		}
	}

	return nil, models.ErrAppsNotFound
}

type sortA []*models.App

func (s sortA) Len() int           { return len(s) }
func (s sortA) Less(i, j int) bool { return strings.Compare(s[i].Name, s[j].Name) < 0 }
func (s sortA) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetApps(ctx context.Context, appFilter *models.AppFilter) ([]*models.App, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	sort.Sort(sortA(m.Apps))

	var apps []*models.App
	for _, a := range m.Apps {
		if len(apps) == appFilter.PerPage {
			break
		}
		if strings.Compare(appFilter.Cursor, a.Name) < 0 {
			apps = append(apps, a)
		}
	}

	return apps, nil
}

func (m *mock) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if a, _ := m.GetApp(ctx, app.Name); a != nil {
		return nil, models.ErrAppsAlreadyExists
	}
	m.Apps = append(m.Apps, app)
	return app, nil
}

func (m *mock) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	a, err := m.GetApp(ctx, app.Name)
	if err != nil {
		return nil, err
	}
	a.UpdateConfig(app.Config)

	return a.Clone(), nil
}

func (m *mock) RemoveApp(ctx context.Context, appName string) error {
	m.batchDeleteCalls(ctx, appName)
	m.batchDeleteRoutes(ctx, appName)
	for i, a := range m.Apps {
		if a.Name == appName {
			m.Apps = append(m.Apps[:i], m.Apps[i+1:]...)
			return nil
		}
	}
	return models.ErrAppsNotFound
}

func (m *mock) GetRoute(ctx context.Context, appName, routePath string) (*models.Route, error) {
	for _, r := range m.Routes {
		if r.AppName == appName && r.Path == routePath {
			return r, nil
		}
	}
	return nil, models.ErrRoutesNotFound
}

type sortR []*models.Route

func (s sortR) Len() int           { return len(s) }
func (s sortR) Less(i, j int) bool { return strings.Compare(s[i].Path, s[j].Path) < 0 }
func (s sortR) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetRoutesByApp(ctx context.Context, appName string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	sort.Sort(sortR(m.Routes))

	for _, r := range m.Routes {
		if len(routes) == routeFilter.PerPage {
			break
		}

		if r.AppName == appName &&
			//strings.HasPrefix(r.Path, routeFilter.PathPrefix) && // TODO
			(routeFilter.Image == "" || routeFilter.Image == r.Image) &&
			strings.Compare(routeFilter.Cursor, r.Path) < 0 {

			routes = append(routes, r)
		}
	}
	return
}

func (m *mock) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if _, err := m.GetApp(ctx, route.AppName); err != nil {
		return nil, err
	}

	if r, _ := m.GetRoute(ctx, route.AppName, route.Path); r != nil {
		return nil, models.ErrRoutesAlreadyExists
	}
	m.Routes = append(m.Routes, route)
	return route, nil
}

func (m *mock) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	r, err := m.GetRoute(ctx, route.AppName, route.Path)
	if err != nil {
		return nil, err
	}
	clone := r.Clone()
	clone.Update(route)
	err = clone.Validate()
	if err != nil {
		return nil, err
	}
	r.Update(route) // only if validate works (pointer)
	return clone, nil
}

func (m *mock) RemoveRoute(ctx context.Context, appName, routePath string) error {
	for i, r := range m.Routes {
		if r.AppName == appName && r.Path == routePath {
			m.Routes = append(m.Routes[:i], m.Routes[i+1:]...)
			return nil
		}
	}
	return models.ErrRoutesNotFound
}

func (m *mock) Put(ctx context.Context, key, value []byte) error {
	if len(value) == 0 {
		delete(m.data, string(key))
	} else {
		m.data[string(key)] = value
	}
	return nil
}

func (m *mock) Get(ctx context.Context, key []byte) ([]byte, error) {
	return m.data[string(key)], nil
}

func (m *mock) InsertCall(ctx context.Context, call *models.Call) error {
	m.Calls = append(m.Calls, call)
	return nil
}

func (m *mock) UpdateCallStatus(ctx context.Context, appName, callID, status string, completedAt strfmt.DateTime) error {
	for i, oldCall := range m.Calls {
		if oldCall.ID == callID && oldCall.AppName == appName {
			m.Calls[i].Status = status
			m.Calls[i].CompletedAt = completedAt
			return nil
		}
	}
	return models.ErrCallNotFound
}

func (m *mock) UpdateCallMetrics(ctx context.Context, appName, callID string, CPUUsage, MemoryUsage uint64) error {
	for i, oldCall := range m.Calls {
		if oldCall.ID == callID && oldCall.AppName == appName {
			m.Calls[i].CPUUsage = CPUUsage
			m.Calls[i].MemoryUsage = MemoryUsage
			return nil
		}
	}
	return models.ErrCallNotFound
}

func (m *mock) GetCall(ctx context.Context, appName, callID string) (*models.Call, error) {
	for _, t := range m.Calls {
		if t.ID == callID && t.AppName == appName {
			return t, nil
		}
	}

	return nil, models.ErrCallNotFound
}

type sortC []*models.Call

func (s sortC) Len() int           { return len(s) }
func (s sortC) Less(i, j int) bool { return strings.Compare(s[i].ID, s[j].ID) < 0 }
func (s sortC) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetCalls(ctx context.Context, filter *models.CallFilter) ([]*models.Call, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	// calls are in DESC order so use sort.Reverse
	sort.Sort(sort.Reverse(sortC(m.Calls)))

	var calls []*models.Call
	for _, c := range m.Calls {
		if len(calls) == filter.PerPage {
			break
		}

		if (filter.AppName == "" || c.AppName == filter.AppName) &&
			(filter.Path == "" || filter.Path == c.Path) &&
			(time.Time(filter.FromTime).IsZero() || time.Time(filter.FromTime).Before(time.Time(c.CreatedAt))) &&
			(time.Time(filter.ToTime).IsZero() || time.Time(c.CreatedAt).Before(time.Time(filter.ToTime))) &&
			(filter.Cursor == "" || strings.Compare(filter.Cursor, c.ID) > 0) {

			calls = append(calls, c)
		}
	}

	return calls, nil
}

func (m *mock) batchDeleteCalls(ctx context.Context, appName string) error {
	newCalls := []*models.Call{}
	for _, c := range m.Calls {
		if c.AppName != appName {
			newCalls = append(newCalls, c)
		}
	}
	m.Calls = newCalls
	return nil
}

func (m *mock) batchDeleteRoutes(ctx context.Context, appName string) error {
	newRoutes := []*models.Route{}
	for _, c := range m.Routes {
		if c.AppName != appName {
			newRoutes = append(newRoutes, c)
		}
	}
	m.Routes = newRoutes
	return nil
}

// GetDatabase returns nil here since shouldn't really be used
func (m *mock) GetDatabase() *sqlx.DB {
	return nil
}
