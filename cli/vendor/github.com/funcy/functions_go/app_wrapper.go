package functions

type AppWrapper struct {

	App App `json:"app,omitempty"`

	Error_ ErrorBody `json:"error,omitempty"`
}
