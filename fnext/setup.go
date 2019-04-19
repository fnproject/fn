package fnext

import (
	"net/http"

	"github.com/fnproject/fn/api/models"
)

// Extension is the interface that all extensions must implement in order
// to configure themselves against an ExtServer.
type Extension interface {
	Name() string
	Setup(s ExtServer) error
}

// ExtServer limits what the extension should do and prevents dependency loop, it can be
// used to alter / modify / add the behavior of fn server.
type ExtServer interface {
	// AddAppListener adds a listener that will be invoked around any relevant methods.
	AddAppListener(listener AppListener)
	// AddCallListener adds a listener that will be invoked around any call invocations.
	AddCallListener(listener CallListener)

	// AddAPIMiddleware add middleware
	AddAPIMiddleware(m Middleware)
	// AddAPIMiddlewareFunc add middlewarefunc
	AddAPIMiddlewareFunc(m MiddlewareFunc)
	// AddRootMiddleware add middleware add middleware for end user applications
	AddRootMiddleware(m Middleware)
	// AddRootMiddlewareFunc add middleware for end user applications
	AddRootMiddlewareFunc(m MiddlewareFunc)

	// AddEndpoint adds an endpoint to /v2/x
	AddEndpoint(method, path string, handler APIHandler)
	// AddEndpoint adds an endpoint to /v2/x
	AddEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request))

	// Datastore returns the Datastore Fn is using
	Datastore() models.Datastore
}
