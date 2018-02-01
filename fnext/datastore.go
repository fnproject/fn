package fnext

import (
	"context"

	"github.com/fnproject/fn/api/models"
)

func NewDatastore(ds models.Datastore, al AppListener) models.Datastore {
	return &extds{
		Datastore: ds,
		al:        al,
	}
}

type extds struct {
	models.Datastore
	al AppListener
}

func (e *extds) GetApp(ctx context.Context, appName string) (*models.App, error) {
	err := e.al.BeforeAppGet(ctx, appName)
	if err != nil {
		return nil, err
	}

	app, err := e.Datastore.GetApp(ctx, appName)
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
	err := e.al.BeforeAppDelete(ctx, appName)
	if err != nil {
		return err
	}

	err = e.Datastore.RemoveApp(ctx, appName)
	if err != nil {
		return err
	}

	return e.al.AfterAppDelete(ctx, appName)
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
