package functions

type App struct {

	// Name of this app. Must be different than the image name. Can ony contain alphanumeric, -, and _.
	Name string `json:"name,omitempty"`

	// Application configuration
	Config map[string]string `json:"config,omitempty"`
}
