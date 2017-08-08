package models

import (
	strfmt "github.com/go-openapi/strfmt"
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

// TODO this should either be Task, or should be removed in favor of Task
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

// Task is a representation of a specific invocation of a route.
type Task struct {
	IDStatus

	// App this task belongs to.
	AppName string `json:"app_name"`

	// Path of the route that is responsible for this task
	Path string `json:"path"`

	// Name of Docker image to use.
	Image string `json:"image"`

	// Number of seconds to wait before queueing the task for consumption for the first time. Must be a positive integer. Tasks with a delay start in state "delayed" and transition to "running" after delay seconds.
	Delay int32 `json:"delay,omitempty"`

	// Payload for the task. This is only used by async tasks, to store their input.
	Payload string `json:"payload,omitempty"`

	// Priority of the task. Higher has more priority. 3 levels from 0-2. Tasks at same priority are processed in FIFO order.
	Priority *int32 `json:"priority"`

	// Maximum runtime in seconds.
	Timeout int32 `json:"timeout,omitempty"`

	// Hot function idle timeout in seconds before termination.
	IdleTimeout int32 `json:"idle_timeout,omitempty"`

	// Memory is the amount of RAM this task is allocated.
	Memory uint64 `json:"memory,omitempty"`

	// BaseEnv are the env vars for hot containers, not request specific.
	BaseEnv map[string]string `json:"base_env,omitempty"`

	// Env vars for the task. Comes from the ones set on the Route.
	EnvVars map[string]string `json:"env_vars,omitempty"`

	// Format is the format to pass input into the function.
	// TODO plumb this in async land
	// Format string `json:"format,omitempty"`

	// Time when task completed, whether it was successul or failed. Always in UTC.
	CompletedAt strfmt.DateTime `json:"completed_at,omitempty"`

	// Time when task was submitted. Always in UTC.
	CreatedAt strfmt.DateTime `json:"created_at,omitempty"`

	// Time when task started execution. Always in UTC.
	StartedAt strfmt.DateTime `json:"started_at,omitempty"`
}

type CallFilter struct {
	Path    string
	AppName string
}
