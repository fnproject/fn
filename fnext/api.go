package fnext

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
)

type ApiHandlerFunc func(w http.ResponseWriter, r *http.Request)

// ServeHTTP calls f(w, r).
func (f ApiHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}

type ApiHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type ApiAppHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App)
}

type ApiAppHandlerFunc func(w http.ResponseWriter, r *http.Request, app *models.App)

// ServeHTTP calls f(w, r).
func (f ApiAppHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App) {
	f(w, r, app)
}

type ApiRouteHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route)
}

type ApiRouteHandlerFunc func(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route)

// ServeHTTP calls f(w, r).
func (f ApiRouteHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route) {
	f(w, r, app, route)
}
