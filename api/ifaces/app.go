package ifaces

import "net/http"

type App interface {
	Name() string
	Routes() Route
	Validate() error
}

type Route interface {
	// AppName() string      `json:"appname"`
	Path() string
	Image() string
	Headers() http.Header
}
