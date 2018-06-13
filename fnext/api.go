package fnext

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
)

// APIHandlerFunc is a convenience to make an APIHandler.
type APIHandlerFunc func(w http.ResponseWriter, r *http.Request)

// ServeHTTP calls f(w, r).
func (f APIHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}

// APIHandler may be used to add an http endpoint on the versioned route of the Fn API.
type APIHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// APIAppHandler may be used to add an http endpoint on the versioned route of fn API,
// at /:version/apps/:app
type APIAppHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App)
}

// APIAppHandlerFunc is a convenience for getting an APIAppHandler.
type APIAppHandlerFunc func(w http.ResponseWriter, r *http.Request, app *models.App)

// ServeHTTP calls f(w, r).
func (f APIAppHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App) {
	f(w, r, app)
}

// APIRouteHandler may be used to add an http endpoint on the versioned route of fn API,
// at /:version/apps/:app/routes/:route
type APIRouteHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route)
}

// APIRouteHandlerFunc is a convenience for getting an APIRouteHandler.
type APIRouteHandlerFunc func(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route)

// ServeHTTP calls f(w, r).
func (f APIRouteHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route) {
	f(w, r, app, route)
}
