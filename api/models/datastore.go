package models

type Datastore interface {
	GetApp(appName string) (*App, error)
	GetApps(*AppFilter) ([]*App, error)
	StoreApp(*App) (*App, error)
	RemoveApp(appName string) error

	GetRoute(appName, routeName string) (*Route, error)
	GetRoutes(*RouteFilter) (routes []*Route, err error)
	StoreRoute(*Route) (*Route, error)
	RemoveRoute(appName, routeName string) error
}

func ApplyAppFilter(app *App, filter *AppFilter) bool {
	return true
}

func ApplyRouteFilter(route *Route, filter *RouteFilter) bool {
	return true
}
