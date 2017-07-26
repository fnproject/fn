package functions

type RouteWrapper struct {

	Message string `json:"message,omitempty"`

	Error_ ErrorBody `json:"error,omitempty"`

	Route Route `json:"route,omitempty"`
}
