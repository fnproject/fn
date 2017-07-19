package server

import (
	"context"

	"gitlab-odx.oracle.com/odx/functions/api/models"
)

// AppListener is an interface used to inject custom code at key points in app lifecycle.
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

// AddAppListener adds a listener that will be notified on App created.
func (s *Server) AddAppListener(listener AppListener) {
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
