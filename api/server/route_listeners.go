package server

import (
	"context"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
)

type routeListeners []fnext.RouteListener

var _ fnext.RouteListener = new(routeListeners)

func (s *Server) AddRouteListener(listener fnext.RouteListener) {
	*s.routeListeners = append(*s.routeListeners, listener)
}

func (a *routeListeners) BeforeRouteCreate(ctx context.Context, route *models.Route) error {
	for _, l := range *a {
		err := l.BeforeRouteCreate(ctx, route)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *routeListeners) AfterRouteCreate(ctx context.Context, route *models.Route) error {
	for _, l := range *a {
		err := l.AfterRouteCreate(ctx, route)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *routeListeners) BeforeRouteUpdate(ctx context.Context, route *models.Route) error {
	for _, l := range *a {
		err := l.BeforeRouteUpdate(ctx, route)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *routeListeners) AfterRouteUpdate(ctx context.Context, route *models.Route) error {
	for _, l := range *a {
		err := l.AfterRouteUpdate(ctx, route)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *routeListeners) BeforeRouteDelete(ctx context.Context, appName string, routePath string) error {
	for _, l := range *a {
		err := l.BeforeRouteDelete(ctx, appName, routePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *routeListeners) AfterRouteDelete(ctx context.Context, appName string, routePath string) error {
	for _, l := range *a {
		err := l.AfterRouteDelete(ctx, appName, routePath)
		if err != nil {
			return err
		}
	}
	return nil
}
