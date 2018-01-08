package server

import (
	"context"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
)

// AddAppListener adds a listener that will be notified on App created.
func (s *Server) AddAppListener(listener fnext.AppListener) {
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

// FireBeforeAppGet runs AppListener's BeforeAppGet method.
// todo: All of these listener methods could/should return the 2nd param rather than modifying in place. For instance,
// if a listener were to change the appName here (maybe prefix it or something for the database), it wouldn't be reflected anywhere else.
// If this returned appName, then keep passing along the returned appName, it would work.
func (s *Server) FireBeforeAppGet(ctx context.Context, appName string) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppGet(ctx, appName)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireAfterAppGet runs AppListener's AfterAppGet method.
func (s *Server) FireAfterAppGet(ctx context.Context, app *models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppGet(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireBeforeAppsList runs AppListener's BeforeAppsList method.
func (s *Server) FireBeforeAppsList(ctx context.Context, filter *models.AppFilter) error {
	for _, l := range s.appListeners {
		err := l.BeforeAppsList(ctx, filter)
		if err != nil {
			return err
		}
	}
	return nil
}

// FireAfterAppsList runs AppListener's AfterAppsList method.
func (s *Server) FireAfterAppsList(ctx context.Context, apps []*models.App) error {
	for _, l := range s.appListeners {
		err := l.AfterAppsList(ctx, apps)
		if err != nil {
			return err
		}
	}
	return nil
}
