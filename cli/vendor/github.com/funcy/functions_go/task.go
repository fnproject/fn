package functions

import (
	"time"
)

type Task struct {

	// Name of Docker image to use. This is optional and can be used to override the image defined at the group level.
	Image string `json:"image,omitempty"`

	// Payload for the task. This is what you pass into each task to make it do something.
	Payload string `json:"payload,omitempty"`

	// Group this task belongs to.
	GroupName string `json:"group_name,omitempty"`

	// The error message, if status is 'error'. This is errors due to things outside the task itself. Errors from user code will be found in the log.
	Error_ string `json:"error,omitempty"`

	// Machine usable reason for task being in this state. Valid values for error status are `timeout | killed | bad_exit`. Valid values for cancelled status are `client_request`. For everything else, this is undefined. 
	Reason string `json:"reason,omitempty"`

	// Time when task was submitted. Always in UTC.
	CreatedAt time.Time `json:"created_at,omitempty"`

	// Time when task started execution. Always in UTC.
	StartedAt time.Time `json:"started_at,omitempty"`

	// Time when task completed, whether it was successul or failed. Always in UTC.
	CompletedAt time.Time `json:"completed_at,omitempty"`

	// If this field is set, then this task is a retry of the ID in this field.
	RetryOf string `json:"retry_of,omitempty"`

	// If this field is set, then this task was retried by the task referenced in this field.
	RetryAt string `json:"retry_at,omitempty"`

	// Env vars for the task. Comes from the ones set on the Group.
	EnvVars map[string]string `json:"env_vars,omitempty"`
}
