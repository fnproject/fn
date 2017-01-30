package server

import (
	"context"

	"github.com/iron-io/functions/api/models"
)

type AppListener interface {
	// BeforeAppCreate called right before creating App in the database
	BeforeAppCreate(ctx context.Context, app *models.App) error
	// AfterAppCreate called after creating App in the database
	AfterAppCreate(ctx context.Context, app *models.App) error
	// BeforeAppUpdate called right before updating App in the database
	BeforeAppUpdate(ctx context.Context, app *models.App) error
	// AfterAppUpdate called after updating App in the database
	AfterAppUpdate(ctx context.Context, app *models.App) error
	// BeforeAppDelete called right before deleting App in the database
	BeforeAppDelete(ctx context.Context, app *models.App) error
	// AfterAppDelete called after deleting App in the database
	AfterAppDelete(ctx context.Context, app *models.App) error
}

// AddAppCreateListener adds a listener that will be notified on App created.
func (s *Server) AddAppListener(listener AppListener) {
	s.appListeners = append(s.appListeners, listener)
}

func (s *Server) FireBeforeAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireBeforeAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireBeforeAppDelete(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppDelete(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppDelete(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppDelete(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}
