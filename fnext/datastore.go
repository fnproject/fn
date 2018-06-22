package fnext

import (
	"context"

	"github.com/fnproject/fn/api/models"
)

func NewDatastore(ds models.Datastore, al AppListener, rl RouteListener, fl FnListener) models.Datastore {
	return &extds{
		Datastore: ds,
		al:        al,
		rl:        rl,
		fl:        fl,
	}
}

type extds struct {
	models.Datastore
	al AppListener
	rl RouteListener
	fl FnListener
	tl TriggerListener
}

func (e *extds) InsertTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {
	err := e.tl.BeforeTriggerCreate(ctx, trigger)
	if err != nil {
		return nil, err
	}

	t, err := e.Datastore.InsertTrigger(ctx, trigger)
	if err != nil {
		return nil, err
	}

	err = e.tl.AfterTriggerCreate(ctx, t)
	return t, err
}

func (e *extds) UpdateTrigger(ctx context.Context, trigger *models.Trigger) (*models.Trigger, error) {
	err := e.tl.BeforeTriggerUpdate(ctx, trigger)
	if err != nil {
		return nil, err
	}

	t, err := e.Datastore.UpdateTrigger(ctx, trigger)
	if err != nil {
		return nil, err
	}

	err = e.tl.AfterTriggerUpdate(ctx, t)
	return t, err
}

func (e *extds) RemoveTrigger(ctx context.Context, triggerID string) error {
	err := e.tl.BeforeTriggerDelete(ctx, triggerID)
	if err != nil {
		return err
	}

	err = e.Datastore.RemoveTrigger(ctx, triggerID)
	if err != nil {
		return err
	}

	err = e.tl.AfterTriggerDelete(ctx, triggerID)
	return err
}

func (e *extds) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	err := e.al.BeforeAppGet(ctx, appID)
	if err != nil {
		return nil, err
	}

	app, err := e.Datastore.GetAppByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	err = e.al.AfterAppGet(ctx, app)
	return app, err
}

func (e *extds) InsertApp(ctx context.Context, app *models.App) (*models.App, error) {
	err := e.al.BeforeAppCreate(ctx, app)
	if err != nil {
		return nil, err
	}

	app, err = e.Datastore.InsertApp(ctx, app)
	if err != nil {
		return nil, err
	}

	err = e.al.AfterAppCreate(ctx, app)
	return app, err
}

func (e *extds) UpdateApp(ctx context.Context, app *models.App) (*models.App, error) {
	err := e.al.BeforeAppUpdate(ctx, app)
	if err != nil {
		return nil, err
	}

	app, err = e.Datastore.UpdateApp(ctx, app)
	if err != nil {
		return nil, err
	}

	err = e.al.AfterAppUpdate(ctx, app)
	return app, err
}

func (e *extds) RemoveApp(ctx context.Context, appName string) error {
	var app models.App
	app.Name = appName
	err := e.al.BeforeAppDelete(ctx, &app)
	if err != nil {
		return err
	}

	err = e.Datastore.RemoveApp(ctx, appName)
	if err != nil {
		return err
	}

	return e.al.AfterAppDelete(ctx, &app)
}

func (e *extds) GetApps(ctx context.Context, filter *models.AppFilter) ([]*models.App, error) {
	err := e.al.BeforeAppsList(ctx, filter)
	if err != nil {
		return nil, err
	}

	apps, err := e.Datastore.GetApps(ctx, filter)
	if err != nil {
		return nil, err
	}

	err = e.al.AfterAppsList(ctx, apps)
	return apps, err
}

func (e *extds) InsertRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	err := e.rl.BeforeRouteCreate(ctx, route)
	if err != nil {
		return nil, err
	}

	route, err = e.Datastore.InsertRoute(ctx, route)
	if err != nil {
		return nil, err
	}

	err = e.rl.AfterRouteCreate(ctx, route)
	return route, err
}

func (e *extds) UpdateRoute(ctx context.Context, route *models.Route) (*models.Route, error) {
	err := e.rl.BeforeRouteUpdate(ctx, route)
	if err != nil {
		return nil, err
	}

	route, err = e.Datastore.UpdateRoute(ctx, route)
	if err != nil {
		return nil, err
	}

	err = e.rl.AfterRouteUpdate(ctx, route)
	return route, err
}

func (e *extds) RemoveRoute(ctx context.Context, appName string, routePath string) error {
	err := e.rl.BeforeRouteDelete(ctx, appName, routePath)
	if err != nil {
		return err
	}
	err = e.Datastore.RemoveRoute(ctx, appName, routePath)
	if err != nil {
		return err
	}
	return e.rl.AfterRouteDelete(ctx, appName, routePath)
}

func (e *extds) InsertFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	err := e.fl.BeforeFnCreate(ctx, fn)
	if err != nil {
		return nil, err
	}

	f, err := e.Datastore.InsertFn(ctx, fn)
	if err != nil {
		return nil, err
	}

	err = e.fl.AfterFnCreate(ctx, fn)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (e *extds) UpdateFn(ctx context.Context, fn *models.Fn) (*models.Fn, error) {
	err := e.fl.BeforeFnUpdate(ctx, fn)
	if err != nil {
		return nil, err
	}

	f, err := e.Datastore.UpdateFn(ctx, fn)
	if err != nil {
		return nil, err
	}

	err = e.fl.AfterFnUpdate(ctx, fn)
	if err != nil {
		return nil, err
	}
	return f, nil

}

func (e *extds) RemoveFn(ctx context.Context, fnID string) error {
	err := e.fl.BeforeFnDelete(ctx, fnID)

	if err != nil {
		return err
	}

	err = e.Datastore.RemoveFn(ctx, fnID)
	if err != nil {
		return err
	}

	err = e.fl.AfterFnDelete(ctx, fnID)
	if err != nil {
		return err
	}
	return nil
}
