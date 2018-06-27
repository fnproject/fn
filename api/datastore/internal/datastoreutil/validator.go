package datastoreutil

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/models"
)

// NewValidator returns a models.Datastore which validates certain arguments before delegating to ds.
func NewValidator(ds models.Datastore) models.Datastore {
	return &validator{ds}
}

type validator struct {
	models.Datastore
}

func (v *validator) GetAppID(ctx context.Context, appName string) (string, error) {
	if appName == "" {
		return "", models.ErrAppsMissingName
	}
	return v.Datastore.GetAppID(ctx, appName)
}

func (v *validator) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	if appID == "" {
		return nil, models.ErrAppsMissingID
	}

	return v.Datastore.GetAppByID(ctx, appID)
}

// app and app.Name will never be nil/empty.
func (v *validator) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.ID != "" {
		return nil, models.ErrAppIDProvided
	}
	if err := app.Validate(); err != nil {
		return nil, err
	}

	return v.Datastore.InsertApp(ctx, app)
}

// app and app.Name will never be nil/empty.
func (v *validator) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	if app == nil {
		return nil, models.ErrDatastoreEmptyApp
	}
	if app.ID == "" {
		return nil, models.ErrAppsMissingID
	}

	return v.Datastore.UpdateApp(ctx, app)
}

// name will never be empty.
func (v *validator) RemoveApp(ctx context.Context, appID string) error {
	if appID == "" {
		return models.ErrAppsMissingID
	}

	return v.Datastore.RemoveApp(ctx, appID)
}

// appName and routePath will never be empty.
func (v *validator) GetRoute(ctx context.Context, appID, routePath string) (*models.Route, error) {
	if appID == "" {
		return nil, models.ErrRoutesMissingAppID
	}
	if routePath == "" {
		return nil, models.ErrRoutesMissingPath
	}

	return v.Datastore.GetRoute(ctx, appID, routePath)
}

// appName will never be empty
func (v *validator) GetRoutesByApp(ctx context.Context, appID string, routeFilter *models.RouteFilter) (routes []*models.Route, err error) {
	if appID == "" {
		return nil, models.ErrRoutesMissingAppID
	}

	return v.Datastore.GetRoutesByApp(ctx, appID, routeFilter)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	if route == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}

	if err := route.Validate(); err != nil {
		return nil, err
	}

	return v.Datastore.InsertRoute(ctx, route)
}

// route will never be nil and route's AppName and Path will never be empty.
func (v *validator) UpdateRoute(ctx context.Context, newroute *models.Route) (*models.Route, error) {
	if newroute == nil {
		return nil, models.ErrDatastoreEmptyRoute
	}
	if newroute.AppID == "" {
		return nil, models.ErrRoutesMissingAppID
	}
	if newroute.Path == "" {
		return nil, models.ErrRoutesMissingPath
	}
	return v.Datastore.UpdateRoute(ctx, newroute)
}

// appName and routePath will never be empty.
func (v *validator) RemoveRoute(ctx context.Context, appID string, routePath string) error {
	if appID == "" {
		return models.ErrRoutesMissingAppID
	}
	if routePath == "" {
		return models.ErrRoutesMissingPath
	}

	return v.Datastore.RemoveRoute(ctx, appID, routePath)
}

func (v *validator) InsertTrigger(ctx context.Context, t *models.Trigger) (*models.Trigger, error) {

	if t.ID != "" {
		return nil, models.ErrTriggerIDProvided
	}

	if !time.Time(t.CreatedAt).IsZero() {
		return nil, models.ErrCreatedAtProvided
	}
	if !time.Time(t.UpdatedAt).IsZero() {
		return nil, models.ErrUpdatedAtProvided
	}

	return v.Datastore.InsertTrigger(ctx, t)
}

func (v *validator) GetTriggers(ctx context.Context, filter *models.TriggerFilter) ([]*models.Trigger, error) {

	if filter.AppID == "" {
		return nil, models.ErrTriggerMissingAppID
	}

	return v.Datastore.GetTriggers(ctx, filter)
}
func (v *validator) RemoveTrigger(ctx context.Context, triggerID string) error {
	if triggerID == "" {
		return models.ErrMissingID
	}

	return v.Datastore.RemoveTrigger(ctx, triggerID)
}

func (v *validator) InsertFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	if fn == nil {
		return nil, models.ErrDatastoreEmptyFn
	}
	if fn.ID != "" {
		return nil, models.ErrFnsIDProvided
	}
	if fn.AppID == "" {
		return nil, models.ErrFnsMissingAppID
	}
	if fn.Name == "" {
		return nil, models.ErrFnsMissingName
	}
	return v.Datastore.InsertFn(ctx, fn)
}

func (v *validator) GetFnByID(ctx context.Context, fnID string) (*models.Fn, error) {
	if fnID == "" {
		return nil, models.ErrDatastoreEmptyFnID
	}

	return v.Datastore.GetFnByID(ctx, fnID)
}

func (v *validator) GetFns(ctx context.Context, filter *models.FnFilter) (*models.FnList, error) {

	if filter.AppID == "" {
		return nil, models.ErrFnsMissingAppID
	}

	return v.Datastore.GetFns(ctx, filter)
}

func (v *validator) RemoveFn(ctx context.Context, fnID string) error {
	if fnID == "" {
		return models.ErrDatastoreEmptyFnID
	}
	return v.Datastore.RemoveFn(ctx, fnID)
}
