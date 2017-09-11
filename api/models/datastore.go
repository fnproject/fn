package models

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Datastore interface {
	// GetApp gets an App by name.
	// Returns ErrDatastoreEmptyAppName for empty appName.
	// Returns ErrAppsNotFound if no app is found.
	GetApp(ctx context.Context, appName string) (*App, error)

	// GetApps gets a slice of Apps, optionally filtered by name.
	// Missing filter or empty name will match all Apps.
	GetApps(ctx context.Context, filter *AppFilter) ([]*App, error)

	// InsertApp inserts an App. Returns ErrDatastoreEmptyApp when app is nil, and
	// ErrDatastoreEmptyAppName when app.Name is empty.
	// Returns ErrAppsAlreadyExists if an App by the same name already exists.
	InsertApp(ctx context.Context, app *App) (*App, error)

	// UpdateApp updates an App's Config. Returns ErrDatastoreEmptyApp when app is nil, and
	// ErrDatastoreEmptyAppName when app.Name is empty.
	// Returns ErrAppsNotFound if an App is not found.
	UpdateApp(ctx context.Context, app *App) (*App, error)

	// RemoveApp removes the App named appName. Returns ErrDatastoreEmptyAppName if appName is empty.
	// Returns ErrAppsNotFound if an App is not found.
	// TODO remove routes automatically? #528
	RemoveApp(ctx context.Context, appName string) error

	// GetRoute looks up a the exact route matching a given route path
	// Returns ErrDatastoreEmptyAppName when appName is empty, and ErrDatastoreEmptyRoutePath when
	// routePath is empty.
	// Returns ErrRoutesNotFound when no matching route is found.
	GetRoute(ctx context.Context, appName, routePath string) (*Route, error)

	// MatchRoute finds the most-specific route that matches a given path,
	// this includes wildcard routes with less-specific paths than the given path
	MatchRoute(ctx context.Context, appName, routePath string) (*Route, error)

	// GetRoutes gets a slice of Routes, optionally filtered by filter.
	GetRoutes(ctx context.Context, filter *RouteFilter) ([]*Route, error)

	// GetRoutesByApp gets a slice of routes for a appName, optionally filtering on filter (filter.AppName is ignored).
	// Returns ErrDatastoreEmptyAppName if appName is empty.
	GetRoutesByApp(ctx context.Context, appName string, filter *RouteFilter) ([]*Route, error)

	// InsertRoute inserts a route. Returns ErrDatastoreEmptyRoute when route is nil, and ErrDatastoreEmptyAppName
	// or ErrDatastoreEmptyRoutePath for empty AppName or Path.
	// Returns ErrRoutesAlreadyExists if the exact route.Path already exists
	InsertRoute(ctx context.Context, route *Route) (*Route, error)

	// UpdateRoute updates route's Config and Header fields. Returns ErrDatastoreEmptyRoute when route is nil, and
	// ErrDatastoreEmptyAppName or ErrDatastoreEmptyRoutePath for empty AppName or Path.
	UpdateRoute(ctx context.Context, route *Route) (*Route, error)

	// RemoveRoute removes a route. Returns ErrDatastoreEmptyAppName when appName is empty, and
	// ErrDatastoreEmptyRoutePath when routePath is empty. Returns ErrRoutesNotFound when no route exists.
	RemoveRoute(ctx context.Context, appName, routePath string) error

	// InsertCall inserts a call into the datastore, it will error if the call already
	// exists.
	InsertCall(ctx context.Context, call *Call) error

	// GetCall returns a call at a certain id and app name.
	GetCall(ctx context.Context, appName, callID string) (*Call, error)

	// GetCalls returns a list of calls that satisfy the given CallFilter. If no
	// calls exist, an empty list and a nil error are returned.
	GetCalls(ctx context.Context, filter *CallFilter) ([]*Call, error)

	// Implement LogStore methods for convenience
	LogStore

	// GetDatabase returns the underlying sqlx database implementation
	GetDatabase() *sqlx.DB
}
