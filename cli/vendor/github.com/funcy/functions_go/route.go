package functions

type Route struct {

	// URL path that will be matched to this route
	Path string `json:"path,omitempty"`

	// Name of Docker image to use in this route. You should include the image tag, which should be a version number, to be more accurate. Can be overridden on a per route basis with route.image.
	Image string `json:"image,omitempty"`

	// Map of http headers that will be sent with the response
	Headers map[string][]string `json:"headers,omitempty"`

	// Max usable memory for this route (MiB).
	Memory int64 `json:"memory,omitempty"`

	// Route type
	Type_ string `json:"type,omitempty"`

	// Payload format sent into function.
	Format string `json:"format,omitempty"`

	// Maximum number of hot containers concurrency
	MaxConcurrency int32 `json:"max_concurrency,omitempty"`

	// Route configuration - overrides application configuration
	Config map[string]string `json:"config,omitempty"`

	// Timeout for executions of this route. Value in Seconds
	Timeout int32 `json:"timeout,omitempty"`
}
