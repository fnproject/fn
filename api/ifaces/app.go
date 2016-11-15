package ifaces

import "net/http"

type App interface {
	Name() string
	Routes() Route
	Validate() error
}

type Route interface {
	Path() string
	Image() string
	Headers() http.Header
}
