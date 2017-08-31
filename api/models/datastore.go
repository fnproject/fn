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

	// GetRoute looks up a matching Route for appName and the literal request route routePath.
	// Returns ErrDatastoreEmptyAppName when appName is empty, and ErrDatastoreEmptyRoutePath when
	// routePath is empty.
	// Returns ErrRoutesNotFound when no matching route is found.
	GetRoute(ctx context.Context, appName, routePath string) (*Route, error)

	// GetRoutes gets a slice of Routes, optionally filtered by filter.
	GetRoutes(ctx context.Context, filter *RouteFilter) (routes []*Route, err error)

	// GetRoutesByApp gets a slice of routes for a appName, optionally filtering on filter (filter.AppName is ignored).
	// Returns ErrDatastoreEmptyAppName if appName is empty.
	GetRoutesByApp(ctx context.Context, appName string, filter *RouteFilter) (routes []*Route, err error)

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

	// InsertTask inserts a task
	InsertTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, callID string) (*Task, error)
	GetTasks(ctx context.Context, filter *CallFilter) ([]*Task, error)

	// Implement FnLog methods for convenience
	FnLog

	// GetDatabase returns the underlying sqlx database implementation
	GetDatabase() *sqlx.DB
}
