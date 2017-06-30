package functions

type RoutesWrapper struct {

	Routes []Route `json:"routes,omitempty"`

	Error_ ErrorBody `json:"error,omitempty"`
}
