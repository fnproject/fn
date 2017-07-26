package functions

type NewTask struct {

	// Name of Docker image to use. This is optional and can be used to override the image defined at the group level.
	Image string `json:"image,omitempty"`

	// Payload for the task. This is what you pass into each task to make it do something.
	Payload string `json:"payload,omitempty"`
}
