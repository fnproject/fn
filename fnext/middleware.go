package fnext

import (
	"net/http"
)

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
