package server

import (
	"context"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
)

type fnListeners []fnext.FnListener

var _ fnext.FnListener = new(fnListeners)

func (s *Server) AddFnListener(listener fnext.FnListener) {
	*s.fnListeners = append(*s.fnListeners, listener)
}

func (a *fnListeners) BeforeFnCreate(ctx context.Context, fn *models.Fn) error {
	for _, l := range *a {
		err := l.BeforeFnCreate(ctx, fn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *fnListeners) AfterFnCreate(ctx context.Context, fn *models.Fn) error {
	for _, l := range *a {
		err := l.AfterFnCreate(ctx, fn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *fnListeners) BeforeFnUpdate(ctx context.Context, fn *models.Fn) error {
	for _, l := range *a {
		err := l.BeforeFnUpdate(ctx, fn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *fnListeners) AfterFnUpdate(ctx context.Context, fn *models.Fn) error {
	for _, l := range *a {
		err := l.AfterFnUpdate(ctx, fn)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *fnListeners) BeforeFnDelete(ctx context.Context, appName string, routePath string) error {
	for _, l := range *a {
		err := l.BeforeFnDelete(ctx, appName, routePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *fnListeners) AfterFnDelete(ctx context.Context, appName string, routePath string) error {
	for _, l := range *a {
		err := l.AfterFnDelete(ctx, appName, routePath)
		if err != nil {
			return err
		}
	}
	return nil
}
