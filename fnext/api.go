package fnext

import (
	"net/http"
)

// APIHandlerFunc is a convenience to make an APIHandler.
type APIHandlerFunc func(w http.ResponseWriter, r *http.Request)

// ServeHTTP calls f(w, r).
func (f APIHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}

// APIHandler may be used to add an http endpoint on the versioned route of the Fn API.
type APIHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}
