package server

import (
	"context"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
)

type appListeners []fnext.AppListener

var _ fnext.AppListener = new(appListeners)

// AddAppListener adds an AppListener for the server to use.
func (s *Server) AddAppListener(listener fnext.AppListener) {
	*s.appListeners = append(*s.appListeners, listener)
}

func (a *appListeners) BeforeAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range *a {
		err := l.BeforeAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) AfterAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range *a {
		err := l.AfterAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) BeforeAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range *a {
		err := l.BeforeAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) AfterAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range *a {
		err := l.AfterAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) BeforeAppDelete(ctx context.Context, app *models.App) error {
	for _, l := range *a {
		err := l.BeforeAppDelete(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) AfterAppDelete(ctx context.Context, app *models.App) error {
	for _, l := range *a {
		err := l.AfterAppDelete(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) BeforeAppGet(ctx context.Context, appName string) error {
	for _, l := range *a {
		err := l.BeforeAppGet(ctx, appName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) AfterAppGet(ctx context.Context, app *models.App) error {
	for _, l := range *a {
		err := l.AfterAppGet(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) BeforeAppsList(ctx context.Context, filter *models.AppFilter) error {
	for _, l := range *a {
		err := l.BeforeAppsList(ctx, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *appListeners) AfterAppsList(ctx context.Context, apps []*models.App) error {
	for _, l := range *a {
		err := l.AfterAppsList(ctx, apps)
		if err != nil {
			return err
		}
	}
	return nil
}
