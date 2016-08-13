package models

import "errors"

type Datastore interface {
	GetApp(appName string) (*App, error)
	GetApps(*AppFilter) ([]*App, error)
	StoreApp(*App) (*App, error)
	RemoveApp(appName string) error

	GetRoute(appName, routeName string) (*Route, error)
	GetRoutes(*RouteFilter) (routes []*Route, err error)
	GetRoutesByApp(string, *RouteFilter) (routes []*Route, err error)
	StoreRoute(*Route) (*Route, error)
	RemoveRoute(appName, routeName string) error

	// The following provide a generic key value store for arbitrary data, can be used by extensions to store extra data
	// todo: should we namespace these by app? Then when an app is deleted, it can delete any of this extra data too.
	Put([]byte, []byte) error
	Get([]byte) ([]byte, error)
}

var (
	ErrDatastoreEmptyAppName   = errors.New("Missing app name")
	ErrDatastoreEmptyRouteName = errors.New("Missing route name")
	ErrDatastoreEmptyApp       = errors.New("Missing app")
	ErrDatastoreEmptyRoute     = errors.New("Missing route")
)

func ApplyAppFilter(app *App, filter *AppFilter) bool {
	return true
}

func ApplyRouteFilter(route *Route, filter *RouteFilter) bool {
	return (filter.Path == "" || route.Path == filter.Path) &&
		(filter.AppName == "" || route.AppName == filter.AppName) &&
		(filter.Image == "" || route.Image == filter.Image)
}
