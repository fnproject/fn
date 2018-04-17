package models

import (
	"context"

	"github.com/jmoiron/sqlx"
)

type Datastore interface {
	// GetAppByID gets an App by ID.
	// Returns ErrDatastoreEmptyAppID for empty appID.
	// Returns ErrAppsNotFound if no app is found.
	GetAppByID(ctx context.Context, appID string) (*App, error)

	// GetAppID gets an app ID by app name, ensures if app exists.
	// Returns ErrDatastoreEmptyAppName for empty appName.
	// Returns ErrAppsNotFound if no app is found.
	GetAppID(ctx context.Context, appName string) (string, error)

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
	RemoveApp(ctx context.Context, appID string) error

	// GetRoute looks up a matching Route for appName and the literal request route routePath.
	// Returns ErrDatastoreEmptyAppName when appName is empty, and ErrDatastoreEmptyRoutePath when
	// routePath is empty.
	// Returns ErrRoutesNotFound when no matching route is found.
	GetRoute(ctx context.Context, appID, routePath string) (*Route, error)

	// GetRoutesByApp gets a slice of routes for a appName, optionally filtering on filter (filter.AppName is ignored).
	// Returns ErrDatastoreEmptyAppName if appName is empty.
	GetRoutesByApp(ctx context.Context, appID string, filter *RouteFilter) ([]*Route, error)

	// InsertRoute inserts a route. Returns ErrDatastoreEmptyRoute when route is nil, and ErrDatastoreEmptyAppName
	// or ErrDatastoreEmptyRoutePath for empty AppName or Path.
	// Returns ErrRoutesAlreadyExists if the exact route.Path already exists
	InsertRoute(ctx context.Context, route *Route) (*Route, error)

	// UpdateRoute updates route's Config and Header fields. Returns ErrDatastoreEmptyRoute when route is nil, and
	// ErrDatastoreEmptyAppName or ErrDatastoreEmptyRoutePath for empty AppName or Path.
	UpdateRoute(ctx context.Context, route *Route) (*Route, error)

	// RemoveRoute removes a route. Returns ErrDatastoreEmptyAppID when appName is empty, and
	// ErrDatastoreEmptyRoutePath when routePath is empty. Returns ErrRoutesNotFound when no route exists.
	RemoveRoute(ctx context.Context, appID, routePath string) error

	// Implement LogStore methods for convenience
	LogStore

	// GetDatabase returns the underlying sqlx database implementation
	GetDatabase() *sqlx.DB
}
