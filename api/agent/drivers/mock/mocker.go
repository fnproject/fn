package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
)

func New() drivers.Driver {
	return &Mocker{}
}

type Mocker struct {
	count int
}

func (m *Mocker) Prepare(context.Context, drivers.ContainerTask) (drivers.Cookie, error) {
	return &cookie{m}, nil
}

type cookie struct {
	m *Mocker
}

func (c *cookie) Freeze(context.Context) error {
	return nil
}

func (c *cookie) Unfreeze(context.Context) error {
	return nil
}

func (c *cookie) Close(context.Context) error { return nil }

func (c *cookie) Run(ctx context.Context) (drivers.WaitResult, error) {
	c.m.count++
	if c.m.count%100 == 0 {
		return nil, fmt.Errorf("Mocker error! Bad.")
	}
	return &runResult{
		err:    nil,
		status: "success",
		start:  time.Now(),
	}, nil
}

type runResult struct {
	err    error
	status string
	start  time.Time
}

func (r *runResult) Wait(context.Context) drivers.RunResult { return r }
func (r *runResult) Status() string                         { return r.status }
func (r *runResult) Error() error                           { return r.err }
