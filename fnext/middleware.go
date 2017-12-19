package fnext

import (
	"context"
	"net/http"
)

// MiddlewareController allows a bit more flow control to the middleware, since we multiple paths a request can go down.
// 1) Could be routed towards the API
// 2) Could be routed towards a function
type MiddlewareController interface {

	// CallFunction skips any API routing and goes down the function path
	CallFunction(w http.ResponseWriter, r *http.Request)

	// If function has already been called
	FunctionCalled() bool
}

// GetMiddlewareController returns MiddlewareController from context.
func GetMiddlewareController(ctx context.Context) MiddlewareController {
	// return ctx.(MiddlewareContext)
	v := ctx.Value(MiddlewareControllerKey)
	return v.(MiddlewareController)
}

// Middleware just takes a http.Handler and returns one. So the next middle ware must be called
// within the returned handler or it would be ignored.
type Middleware interface {
	Handle(next http.Handler) http.Handler
}

// MiddlewareFunc is a here to allow a plain function to be a middleware.
type MiddlewareFunc func(next http.Handler) http.Handler

// Handle used to allow middlewarefuncs to be middleware.
func (m MiddlewareFunc) Handle(next http.Handler) http.Handler {
	return m(next)
}
