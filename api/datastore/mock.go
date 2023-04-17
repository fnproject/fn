package datastore

import (
	"context"
	"encoding/base64"
	"sort"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

type mock struct {
	Apps     []*models.App
	Fns      []*models.Fn
	Triggers []*models.Trigger
}

// NewMock creates a new mock datastore
func NewMock() models.Datastore {
	return NewMockInit()
}

var _ models.Datastore = &mock{}

func (m *mock) GetTriggerBySource(ctx context.Context, appID string, triggerType, source string) (*models.Trigger, error) {
	for _, t := range m.Triggers {
		if t.AppID == appID && t.Type == triggerType && t.Source == source {
			return t, nil
		}
	}

	return nil, models.ErrTriggerNotFound
}

// NewMockInit allows specifying certain apps/fns/triggers. args helps break tests less if we change stuff
func NewMockInit(args ...interface{}) models.Datastore {
	var mocker mock
	for _, a := range args {
		switch x := a.(type) {
		case []*models.App:
			mocker.Apps = x
		case []*models.Fn:
			mocker.Fns = x
		case []*models.Trigger:
			mocker.Triggers = x

		default:
			panic("not accounted for data type sent to mock init. add it")
		}
	}
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

func (m *mock) GetApps(ctx context.Context, filter *models.AppFilter) (*models.AppList, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	sort.Sort(sortA(m.Apps))

	var cursor string
	if filter.Cursor != "" {
		s, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return nil, err
		}
		logrus.Error(s)
		cursor = string(s)
	}

	apps := []*models.App{}
	for _, a := range m.Apps {
		if len(apps) == filter.PerPage {
			break
		}
		if strings.Compare(cursor, a.Name) < 0 {
			if filter.Name != "" && filter.Name != a.Name {
				continue
			}
			apps = append(apps, a.Clone())
		}
	}

	var nextCursor string
	if len(apps) > 0 && len(apps) == filter.PerPage {
		last := []byte(apps[len(apps)-1].Name)
		nextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	return &models.AppList{
		NextCursor: nextCursor,
		Items:      apps,
	}, nil
}

func (m *mock) InsertApp(ctx context.Context, newApp *models.App) (*models.App, error) {
	for _, a := range m.Apps {
		if newApp.Name == a.Name {
			return nil, models.ErrAppsAlreadyExists
		}
	}

	app := newApp.Clone()
	app.CreatedAt = common.DateTime(time.Now())
	app.UpdatedAt = app.CreatedAt
	app.ID = id.New().String()

	m.Apps = append(m.Apps, app)
	return app.Clone(), nil
}

func (m *mock) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {

	appID := app.ID
	for idx, a := range m.Apps {
		if a.ID == appID {
			if app.Name != "" && app.Name != a.Name {
				return nil, models.ErrAppsNameImmutable
			}
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
	for i, a := range m.Apps {
		if a.ID == appID {
			var newFns []*models.Fn
			var newTriggers []*models.Trigger
			newApps := append(m.Apps[0:i], m.Apps[i+1:]...)

			for _, fn := range m.Fns {
				if fn.AppID != appID {
					newFns = append(newFns, fn)
				}
			}

			for _, t := range m.Triggers {
				if t.AppID != appID {
					newTriggers = append(newTriggers, t)
				}
			}

			m.Apps = newApps
			m.Triggers = newTriggers
			m.Fns = newFns
			return nil

		}
	}

	return models.ErrAppsNotFound
}

func (m *mock) InsertFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	_, err := m.GetAppByID(ctx, fn.AppID)
	if err != nil {
		return nil, err
	}

	for _, f := range m.Fns {
		if f.ID == fn.ID ||
			(f.AppID == fn.AppID &&
				f.Name == fn.Name) {
			return nil, models.ErrFnsExists
		}
	}
	cl := fn.Clone()
	cl.ID = id.New().String()
	cl.CreatedAt = common.DateTime(time.Now())
	cl.UpdatedAt = cl.CreatedAt
	err = fn.Validate()
	if err != nil {
		return nil, err
	}

	m.Fns = append(m.Fns, cl)

	return cl.Clone(), nil
}

func (m *mock) UpdateFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	// update if exists
	for _, f := range m.Fns {
		if f.ID == fn.ID {
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

func (m *mock) GetFns(ctx context.Context, filter *models.FnFilter) (*models.FnList, error) {
	// sort them all first for cursoring (this is for testing, n is small & mock is not concurrent..)
	sort.Sort(sortF(m.Fns))

	funcs := []*models.Fn{}

	var cursor string
	if filter.Cursor != "" {
		s, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return nil, err
		}
		cursor = string(s)
	}

	for _, f := range m.Fns {
		if filter.PerPage > 0 && len(funcs) == filter.PerPage {
			break
		}

		if strings.Compare(cursor, f.Name) < 0 &&
			(filter.AppID == "" || filter.AppID == f.AppID) &&
			(filter.Name == "" || filter.Name == f.Name) {
			funcs = append(funcs, f)
		}
	}

	var nextCursor string
	if len(funcs) > 0 && len(funcs) == filter.PerPage {
		last := []byte(funcs[len(funcs)-1].Name)
		nextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	return &models.FnList{
		NextCursor: nextCursor,
		Items:      funcs,
	}, nil
}

func (m *mock) GetFnByID(ctx context.Context, fnID string) (*models.Fn, error) {
	for _, f := range m.Fns {
		if f.ID == fnID {
			return f, nil
		}
	}

	return nil, models.ErrFnsNotFound
}

func (m *mock) RemoveFn(ctx context.Context, fnID string) error {
	for i, f := range m.Fns {
		if f.ID == fnID {
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
	fn, err := m.GetFnByID(ctx, trigger.FnID)
	if err != nil {
		return nil, err
	}

	if fn.AppID != trigger.AppID {
		return nil, models.ErrTriggerFnIDNotSameApp
	}

	for _, t := range m.Triggers {
		if t.ID == trigger.ID ||
			(t.AppID == trigger.AppID &&
				t.FnID == trigger.FnID &&
				t.Name == trigger.Name) {
			return nil, models.ErrTriggerExists
		}

		if t.AppID == trigger.AppID &&
			t.Source == trigger.Source &&
			t.Type == trigger.Type {
			return nil, models.ErrTriggerSourceExists
		}
	}

	cl := trigger.Clone()
	cl.CreatedAt = common.DateTime(time.Now())
	cl.UpdatedAt = cl.CreatedAt
	cl.ID = id.New().String()

	err = trigger.Validate()
	if err != nil {
		return nil, err
	}
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
func (s sortT) Less(i, j int) bool { return strings.Compare(s[i].Name, s[j].Name) < 0 }
func (s sortT) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (m *mock) GetTriggers(ctx context.Context, filter *models.TriggerFilter) (*models.TriggerList, error) {
	sort.Sort(sortT(m.Triggers))

	var cursor string
	if filter.Cursor != "" {
		s, err := base64.RawURLEncoding.DecodeString(filter.Cursor)
		if err != nil {
			return nil, err
		}
		cursor = string(s)
	}

	res := []*models.Trigger{}
	for _, t := range m.Triggers {
		if filter.PerPage > 0 && len(res) == filter.PerPage {
			break
		}

		matched := true
		if filter.Cursor != "" && t.Name <= cursor {
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

		if matched {
			res = append(res, t)
		}
	}

	var nextCursor string
	if len(res) > 0 && len(res) == filter.PerPage {
		last := []byte(res[len(res)-1].Name)
		nextCursor = base64.RawURLEncoding.EncodeToString(last)
	}

	return &models.TriggerList{
		NextCursor: nextCursor,
		Items:      res,
	}, nil
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

func (m *mock) Close() error {
	return nil
}
