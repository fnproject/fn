package runner

import (
	"context"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/drivers"
)

// TODO: Move the listener interfaces to another package

type RunListener interface {
	// BeforeRun called before a function run
	BeforeRun(ctx context.Context, task *models.Task) error
	// AfterRun called after a function run
	AfterRun(ctx context.Context, task *models.Task, result drivers.RunResult) error
}

func (r *Runner) AddRunListener(listener RunListener) {
	r.runListeners = append(r.runListeners, listener)
}
