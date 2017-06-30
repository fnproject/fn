package functions

type AppsWrapper struct {

	Apps []App `json:"apps,omitempty"`

	Error_ ErrorBody `json:"error,omitempty"`
}
