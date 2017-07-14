package models

import (
	"encoding/json"

	strfmt "github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
)

const (
	// TypeNone ...
	TypeNone = ""
	// TypeSync ...
	TypeSync = "sync"
	// TypeAsync ...
	TypeAsync = "async"
)

const (
	// FormatDefault ...
	FormatDefault = "default"
	// FormatHTTP ...
	FormatHTTP = "http"
)

type FnCall struct {
	IDStatus
	CompletedAt strfmt.DateTime `json:"completed_at,omitempty"`
	CreatedAt   strfmt.DateTime `json:"created_at,omitempty"`
	StartedAt   strfmt.DateTime `json:"started_at,omitempty"`
	AppName     string          `json:"app_name,omitempty"`
	Path        string          `json:"path"`
}

type FnCallLog struct {
	CallID string `json:"call_id"`
	Log    string `json:"log"`
}

func (fnCall *FnCall) FromTask(task *Task) *FnCall {
	return &FnCall{
		CreatedAt:   task.CreatedAt,
		StartedAt:   task.StartedAt,
		CompletedAt: task.CompletedAt,
		AppName:     task.AppName,
		Path:        task.Path,
		IDStatus: IDStatus{
			ID:     task.ID,
			Status: task.Status,
		},
	}
}

/*Task task

swagger:model Task
*/
type Task struct {
	IDStatus

	/* Number of seconds to wait before queueing the task for consumption for the first time. Must be a positive integer. Tasks with a delay start in state "delayed" and transition to "running" after delay seconds.
	 */
	Delay int32 `json:"delay,omitempty"`

	/* Name of Docker image to use. This is optional and can be used to override the image defined at the route level.

	Required: true
	*/
	Image *string `json:"image"`

	/* "Number of automatic retries this task is allowed.  A retry will be attempted if a task fails. Max 25. Automatic retries are performed by titan when a task reaches a failed state and has `max_retries` > 0. A retry is performed by queueing a new task with the same image id and payload. The new task's max_retries is one less than the original. The new task's `retry_of` field is set to the original Task ID. The old task's `retry_at` field is set to the new Task's ID.  Titan will delay the new task for retries_delay seconds before queueing it. Cancelled or successful tasks are never automatically retried."

	 */
	MaxRetries int32 `json:"max_retries,omitempty"`

	/* Payload for the task. This is what you pass into each task to make it do something.
	 */
	Payload string `json:"payload,omitempty"`

	/* Priority of the task. Higher has more priority. 3 levels from 0-2. Tasks at same priority are processed in FIFO order.

	Required: true
	*/
	Priority *int32 `json:"priority"`

	/* Time in seconds to wait before retrying the task. Must be a non-negative integer.
	 */
	RetriesDelay *int32 `json:"retries_delay,omitempty"`

	/* Maximum runtime in seconds. If a consumer retrieves the
	task, but does not change it's status within timeout seconds, the task
	is considered failed, with reason timeout (Titan may allow a small
	grace period). The consumer should also kill the task after timeout
	seconds. If a consumer tries to change status after Titan has already
	timed out the task, the consumer will be ignored.

	*/
	Timeout *int32 `json:"timeout,omitempty"`

	/* Hot function idle timeout in seconds before termination.

	 */
	IdleTimeout *int32 `json:"idle_timeout,omitempty"`

	/* Time when task completed, whether it was successul or failed. Always in UTC.
	 */
	CompletedAt strfmt.DateTime `json:"completed_at,omitempty"`

	/* Time when task was submitted. Always in UTC.

	Read Only: true
	*/
	CreatedAt strfmt.DateTime `json:"created_at,omitempty"`

	/* Env vars for the task. Comes from the ones set on the Route.
	 */
	EnvVars map[string]string `json:"env_vars,omitempty"`

	/* The error message, if status is 'error'. This is errors due to things outside the task itself. Errors from user code will be found in the log.
	 */
	Error string `json:"error,omitempty"`

	/* App this task belongs to.

	Read Only: true
	*/
	AppName string `json:"app_name,omitempty"`

	Path string `json:"path"`

	/* Machine usable reason for task being in this state.
	Valid values for error status are `timeout | killed | bad_exit`.
	Valid values for cancelled status are `client_request`.
	For everything else, this is undefined.

	*/
	Reason string `json:"reason,omitempty"`

	/* If this field is set, then this task was retried by the task referenced in this field.

	Read Only: true
	*/
	RetryAt string `json:"retry_at,omitempty"`

	/* If this field is set, then this task is a retry of the ID in this field.

	Read Only: true
	*/
	RetryOf string `json:"retry_of,omitempty"`

	/* Time when task started execution. Always in UTC.
	 */
	StartedAt strfmt.DateTime `json:"started_at,omitempty"`
}

// Validate validates this task
func (m *Task) Validate(formats strfmt.Registry) error {
	if err := m.IDStatus.Validate(formats); err != nil {
		return err
	}

	if err := m.validateEnvVars(formats); err != nil {
		return err
	}

	if err := m.validateReason(formats); err != nil {
		return err
	}

	return nil
}

func (m *Task) validateEnvVars(formats strfmt.Registry) error {

	if err := validate.Required("env_vars", "body", m.EnvVars); err != nil {
		return err
	}

	return nil
}

var taskTypeReasonPropEnum []interface{}

// property enum
func (m *Task) validateReasonEnum(path, location string, value string) error {
	if taskTypeReasonPropEnum == nil {
		var res []string
		if err := json.Unmarshal([]byte(`["timeout","killed","bad_exit","client_request"]`), &res); err != nil {
			return err
		}
		for _, v := range res {
			taskTypeReasonPropEnum = append(taskTypeReasonPropEnum, v)
		}
	}
	if err := validate.Enum(path, location, value, taskTypeReasonPropEnum); err != nil {
		return err
	}
	return nil
}

func (m *Task) validateReason(formats strfmt.Registry) error {

	// value enum
	if err := m.validateReasonEnum("reason", "body", m.Reason); err != nil {
		return err
	}

	return nil
}

type CallFilter struct {
	Path    string
	AppName string
}
