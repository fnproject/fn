package server

import (
	"context"
	"github.com/treeder/functions/api/models"
)

type RunnerListener interface {
	// BeforeDispatch called before a function run
	BeforeDispatch(ctx context.Context, route *models.Route) error
	// AfterDispatch called after a function run
	AfterDispatch(ctx context.Context, route *models.Route) error
}

// AddRunListeners adds a listener that will be fired before and after a function run.
func (s *Server) AddRunnerListener(listener RunnerListener) {
	s.runnerListeners = append(s.runnerListeners, listener)
}

func (s *Server) FireBeforeDispatch(ctx context.Context, route *models.Route) error {
	for _, l := range s.runnerListeners {
		err := l.BeforeDispatch(ctx, route)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterDispatch(ctx context.Context, route *models.Route) error {
	for _, l := range s.runnerListeners {
		err := l.AfterDispatch(ctx, route)
		if err != nil {
			return err
		}
	}
	return nil
}
