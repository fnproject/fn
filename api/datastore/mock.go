package datastore

import (
	"context"
	"sort"
	"strings"

	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/jmoiron/sqlx"
)

type mock struct {
	Apps   []*models.App
	Routes []*models.Route

	models.LogStore
}

func NewMock() models.Datastore {
	return NewMockInit()
}

// args helps break tests less if we change stuff
func NewMockInit(args ...interface{}) models.Datastore {
	var mocker mock
	for _, a := range args {
		switch x := a.(type) {
		case []*models.App:
			mocker.Apps = x
		case []*models.Route:
			mocker.Routes = x
		default:
			panic("not accounted for data type sent to mock init. add it")
		}
	}
	mocker.LogStore = logs.NewMock()
	return datastoreutil.NewValidator(&mocker)
}

func (m *mock) GetAppID(ctx context.Context, appName string) (string, error) {
	for _, a := range m.Apps {
		if a.Name == appName {
			return a.ID, nil
		}
	}

	return "", models.ErrAppsNotFound
}

func (m *mock) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	for _, a := range m.Apps {
		if a.ID == appID {
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
	if a, _ := m.GetAppByID(ctx, app.ID); a != nil {
		return nil, models.ErrAppsAlreadyExists
	}
	m.Apps = append(m.Apps, app)
	return app, nil
}

func (m *mock) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	a, err := m.GetAppByID(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	a.Update(app)

	return a.Clone(), nil
}

func (m *mock) RemoveApp(ctx context.Context, appID string) error {
	m.batchDeleteRoutes(ctx, appID)
	for i, a := range m.Apps {
		if a.ID == appID {
			m.Apps = append(m.Apps[:i], m.Apps[i+1:]...)
			return nil
		}
	}
	return models.ErrAppsNotFound
}

func (m *mock) GetRoute(ctx context.Context, appID, routePath string) (*models.Route, error) {
	for _, r := range m.Routes {
		if r.AppID == appID && r.Path == routePath {
			return r, nil
		}
	}
	return nil, models.ErrRoutesNotFound
}

type sortR []*models.Route

func (s sortR) Len() int           { return len(s) }
func (s sortR) Less(i, j int) bool { return strings.Compare(s[i].Path, s[j].Path) < 0 }
func (s sortR) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetRoutesByApp(ctx context.Context, appID string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	sort.Sort(sortR(m.Routes))

	for _, r := range m.Routes {
		if len(routes) == routeFilter.PerPage {
			break
		}

		if r.AppID == appID &&
			//strings.HasPrefix(r.Path, routeFilter.PathPrefix) && // TODO
			(routeFilter.Image == "" || routeFilter.Image == r.Image) &&
			strings.Compare(routeFilter.Cursor, r.Path) < 0 {

			routes = append(routes, r)
		}
	}
	return
}

func (m *mock) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if _, err := m.GetAppByID(ctx, route.AppID); err != nil {
		return nil, err
	}

	if r, _ := m.GetRoute(ctx, route.AppID, route.Path); r != nil {
		return nil, models.ErrRoutesAlreadyExists
	}
	m.Routes = append(m.Routes, route)
	return route, nil
}

func (m *mock) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	r, err := m.GetRoute(ctx, route.AppID, route.Path)
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

func (m *mock) RemoveRoute(ctx context.Context, appID, routePath string) error {
	for i, r := range m.Routes {
		if r.AppID == appID && r.Path == routePath {
			m.Routes = append(m.Routes[:i], m.Routes[i+1:]...)
			return nil
		}
	}
	return models.ErrRoutesNotFound
}

func (m *mock) batchDeleteRoutes(ctx context.Context, appID string) error {
	newRoutes := []*models.Route{}
	for _, c := range m.Routes {
		if c.AppID != appID {
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
