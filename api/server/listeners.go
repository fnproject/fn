package server

import (
	"context"
	"github.com/iron-io/functions/api/ifaces"
	"github.com/iron-io/functions/api/models"
)

// AddAppCreateListener adds a listener that will be notified on App created.
func (s *Server) AddAppCreateListener(listener ifaces.AppCreateListener) {
	s.AppCreateListeners = append(s.AppCreateListeners, listener)
}

// AddAppUpdateListener adds a listener that will be notified on App updated.
func (s *Server) AddAppUpdateListener(listener ifaces.AppUpdateListener) {
	s.AppUpdateListeners = append(s.AppUpdateListeners, listener)
}

// AddAppDeleteListener adds a listener that will be notified on App deleted.
func (s *Server) AddAppDeleteListener(listener ifaces.AppDeleteListener) {
	s.AppDeleteListeners = append(s.AppDeleteListeners, listener)
}

func (s *Server) FireBeforeAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppCreateListeners {
		err := l.BeforeAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppCreateListeners {
		err := l.AfterAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireBeforeAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppUpdateListeners {
		err := l.BeforeAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppUpdateListeners {
		err := l.AfterAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireBeforeAppDelete(ctx context.Context, appName string) error {
	for _, l := range s.AppDeleteListeners {
		err := l.BeforeAppDelete(ctx, appName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppDelete(ctx context.Context, appName string) error {
	for _, l := range s.AppDeleteListeners {
		err := l.AfterAppDelete(ctx, appName)
		if err != nil {
			return err
		}
	}
	return nil
}
