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
	Fns    []*models.Fn

	Triggers []*models.Trigger

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
		case []*models.Fn:
			mocker.Fns = x
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
			return a.Clone(), nil
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
			apps = append(apps, a.Clone())
		}
	}

	return apps, nil
}

func (m *mock) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if a, _ := m.GetAppByID(ctx, app.ID); a != nil {
		return nil, models.ErrAppsAlreadyExists
	}
	m.Apps = append(m.Apps, app)
	return app.Clone(), nil
}

func (m *mock) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	for idx, a := range m.Apps {
		if a.ID == app.ID {
			c := a.Clone()
			c.Update(app)
			err := c.Validate()
			if err != nil {
				return nil, err
			}
			m.Apps[idx] = c
			return c.Clone(), nil
		}
	}

	return nil, models.ErrAppsNotFound

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
			return r.Clone(), nil
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

			routes = append(routes, r.Clone())
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
	return route.Clone(), nil
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
	var newRoutes []*models.Route
	for _, c := range m.Routes {
		if c.AppID != appID {
			newRoutes = append(newRoutes, c)
		}
	}
	m.Routes = newRoutes
	return nil
}

func (m *mock) InsertFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	// update if exists
	for _, f := range m.Fns {
		if f.AppID == fn.AppID && f.Name == fn.Name {
			return nil, models.ErrFnsExists
		}
	}

	app, err := m.GetAppByID(ctx, fn.AppID)
	if err != nil {
		return nil, err
	}
	clone := fn.Clone()
	// insert otherwise
	clone.SetDefaults()
	clone.AppID = app.ID
	if err := clone.Validate(); err != nil {
		return nil, err
	}
	m.Fns = append(m.Fns, clone)
	return clone.Clone(), nil
}

func (m *mock) UpdateFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	// update if exists
	for _, f := range m.Fns {
		if f.AppID == fn.AppID && f.Name == fn.Name {
			if fn.ID != "" && f.ID != fn.ID {
				return nil, models.ErrFnsInvalidFieldChange
			}

			clone := f.Clone()
			clone.Update(fn)
			err := clone.Validate()
			if err != nil {
				return nil, err
			}
			*f = *clone
			return f, nil
		}
	}

	return nil, models.ErrFnsNotFound
}

type sortF []*models.Fn

func (s sortF) Len() int           { return len(s) }
func (s sortF) Less(i, j int) bool { return strings.Compare(s[i].Name, s[j].Name) < 0 }
func (s sortF) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetFns(ctx context.Context, filter *models.FnFilter) ([]*models.Fn, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	sort.Sort(sortF(m.Fns))

	funcs := []*models.Fn{}

	for _, f := range m.Fns {
		if len(funcs) == filter.PerPage {
			break
		}

		if strings.Compare(filter.Cursor, f.Name) < 0 &&
			(filter.Image == "" || filter.Image == f.Image) &&
			(filter.AppID == "" || filter.AppID == f.AppID) {
			funcs = append(funcs, f)
		}

	}
	return funcs, nil
}

func (m *mock) GetFnById(ctx context.Context, fnId string) (*models.Fn, error) {
	for _, f := range m.Fns {
		if f.ID == fnId {
			return f, nil
		}
	}

	return nil, models.ErrFnsNotFound
}
func (m *mock) GetFn(ctx context.Context, appID string, funcName string) (*models.Fn, error) {
	for _, f := range m.Fns {
		if f.Name == funcName && f.AppID == appID {
			return f, nil
		}
	}

	return nil, models.ErrFnsNotFound
}

func (m *mock) RemoveFn(ctx context.Context, appID string, funcName string) error {
	for i, f := range m.Fns {
		if f.Name == funcName && f.AppID == appID {
			m.Fns = append(m.Fns[:i], m.Fns[i+1:]...)
			var newTriggers []*models.Trigger
			for _, t := range m.Triggers {
				if t.FnID != f.ID {
					newTriggers = append(newTriggers, t)
				}
			}

			m.Triggers = newTriggers
			return nil
		}
	}

	return models.ErrFnsNotFound
}

func (m *mock) InsertTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {
	_, err := m.GetAppByID(ctx, trigger.AppID)
	if err != nil {
		return nil, err
	}
	_, err = m.GetFnById(ctx, trigger.FnID)
	if err != nil {
		return nil, err
	}

	for _, t := range m.Triggers {
		if t.ID == trigger.ID ||
			(t.AppID == trigger.AppID &&
				t.FnID == trigger.FnID &&
				t.Name == trigger.Name) {
			return nil, models.ErrTriggerAlreadyExists
		}
	}
	err = trigger.Validate()
	if err != nil {
		return nil, err
	}
	cl := trigger.Clone()
	cl.SetDefaults()
	m.Triggers = append(m.Triggers, cl)

	return cl.Clone(), nil

}

func (m *mock) UpdateTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {
	for _, t := range m.Triggers {
		if t.ID == trigger.ID {
			cl := t.Clone()
			cl.Update(trigger)
			err := cl.Validate()
			if err != nil {
				return nil, err
			}
			*t = *cl
			return cl.Clone(), nil
		}
	}
	return nil, models.ErrTriggerNotFound
}

func (m *mock) GetTrigger(ctx context.Context, appId, fnId, triggerName string) (*models.Trigger, error) {
	for _, t := range m.Triggers {
		if t.AppID == appId && t.FnID == fnId && t.Name == triggerName {
			return t.Clone(), nil
		}
	}
	return nil, models.ErrTriggerNotFound
}

func (m *mock) PutTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {

	if r, _ := m.GetTriggerByID(ctx, trigger.ID); r != nil {
		// update if exists
		return nil, models.ErrTriggerAlreadyExists
	} else {
		cl := trigger.Clone()
		cl.SetDefaults()
		m.Triggers = append(m.Triggers, cl)
		return cl.Clone(), nil
	}
}

func (m *mock) GetTriggerByID(ctx context.Context, triggerId string) (*models.Trigger, error) {
	for _, t := range m.Triggers {
		if t.ID == triggerId {
			return t.Clone(), nil
		}
	}
	return nil, models.ErrTriggerNotFound
}

type sortT []*models.Trigger

func (s sortT) Len() int           { return len(s) }
func (s sortT) Less(i, j int) bool { return strings.Compare(s[i].ID, s[j].ID) < 0 }
func (s sortT) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetTriggers(ctx context.Context, filter *models.TriggerFilter) ([]*models.Trigger, error) {
	sort.Sort(sortT(m.Triggers))

	res := []*models.Trigger{}
	for _, t := range m.Triggers {
		matched := true
		if filter.Cursor != "" && t.ID <= filter.Cursor {
			matched = false
		}
		if t.AppID != filter.AppID {
			matched = false
		}
		if filter.FnID != "" && filter.FnID != t.FnID {
			matched = false
		}
		if filter.Name != "" && filter.Name != t.Name {
			matched = false
		}
		if filter.Type != 0 && filter.Type != t.Type {
			matched = false
		}
		if filter.Source != "" && filter.Source != t.Source {
			matched = false
		}

		if matched {
			res = append(res, t)
		}
	}

	if filter.PerPage != 0 {
		return res[0:filter.PerPage], nil
	}
	return res, nil
}

func (m *mock) RemoveTrigger(ctx context.Context, triggerID string) error {
	for i, t := range m.Triggers {
		if t.ID == triggerID {
			m.Triggers = append(m.Triggers[:i], m.Triggers[i+1:]...)
			return nil
		}
	}
	return models.ErrTriggerNotFound
}

// GetDatabase returns nil here since shouldn't really be used
func (m *mock) GetDatabase() *sqlx.DB {
	return nil
}

func (m *mock) Close() error {
	return nil
}
