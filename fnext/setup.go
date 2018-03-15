package fnext

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
)

type Extension interface {
	Name() string
	Setup(s ExtServer) error
}

// NOTE: ExtServer limits what the extension should do and prevents dependency loop
type ExtServer interface {
	AddAppListener(listener AppListener)
	AddCallListener(listener CallListener)

	// AddAPIMiddleware add middleware
	AddAPIMiddleware(m Middleware)
	// AddAPIMiddlewareFunc add middlewarefunc
	AddAPIMiddlewareFunc(m MiddlewareFunc)
	// AddRootMiddleware add middleware add middleware for end user applications
	AddRootMiddleware(m Middleware)
	// AddRootMiddlewareFunc add middleware for end user applications
	AddRootMiddlewareFunc(m MiddlewareFunc)

	// AddEndpoint adds an endpoint to /v1/x
	AddEndpoint(method, path string, handler ApiHandler)
	// AddEndpoint adds an endpoint to /v1/x
	AddEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request))
	// AddAppEndpoint adds an endpoints to /v1/apps/:app/x
	AddAppEndpoint(method, path string, handler ApiAppHandler)
	// AddAppEndpoint adds an endpoints to /v1/apps/:app/x
	AddAppEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request, app *models.App))
	// AddRouteEndpoint adds an endpoints to /v1/apps/:app/routes/:route/x
	AddRouteEndpoint(method, path string, handler ApiRouteHandler)
	// AddRouteEndpoint adds an endpoints to /v1/apps/:app/routes/:route/x
	AddRouteEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route))

	// WithRunnerPool overrides the default runner pool implementation when running in load-balanced mode
	WithRunnerPool(runnerPool models.RunnerPool)

	// Datastore returns the Datastore Fn is using
	Datastore() models.Datastore
}
