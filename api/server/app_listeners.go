package server

import (
	"context"

	"github.com/fnproject/fn/api/extenders"
	"github.com/fnproject/fn/api/models"
)

// AddAppListener adds a listener that will be notified on App created.
func (s *Server) AddAppListener(listener extenders.AppListener) {
	s.appListeners = append(s.appListeners, listener)
}

// FireBeforeAppCreate is used to call all the server's Listeners BeforeAppCreate functions.
func (s *Server) FireBeforeAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireAfterAppCreate is used to call all the server's Listeners AfterAppCreate functions.
func (s *Server) FireAfterAppCreate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppCreate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireBeforeAppUpdate is used to call all the server's Listeners BeforeAppUpdate functions.
func (s *Server) FireBeforeAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireAfterAppUpdate is used to call all the server's Listeners AfterAppUpdate functions.
func (s *Server) FireAfterAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireBeforeAppDelete is used to call all the server's Listeners BeforeAppDelete functions.
func (s *Server) FireBeforeAppDelete(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppDelete(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireAfterAppDelete is used to call all the server's Listeners AfterAppDelete functions.
func (s *Server) FireAfterAppDelete(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppDelete(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}
