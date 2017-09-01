package server

import (
	"context"
	"github.com/fnproject/fn/api/runner/task"
)

// TaskListener is an interface used to inject custom code at key points in call lifecycle.
type TaskListener interface {
	// BeforeTask called right before the task is started
	BeforeTaskStart(ctx context.Context, task *task.Config) error
}

// AddTaskListener a listener that intercepts a configured task
func (s *Server) AddTaskListener(listener TaskListener) {
	s.taskListeners = append(s.taskListeners, listener)
}

func (s *Server) FireBeforeTaskStart(ctx context.Context, task *task.Config) error {
	for _, l := range s.taskListeners {
		err := l.BeforeTaskStart(ctx, task)
		if err != nil {
			return err
		}
	}
	return nil
}
