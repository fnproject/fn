package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/fnproject/fn/api/runner/drivers"
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

func (c *cookie) Close(context.Context) error { return nil }

func (c *cookie) Run(ctx context.Context) (drivers.RunResult, error) {
	c.m.count++
	if c.m.count%100 == 0 {
		return nil, fmt.Errorf("Mocker error! Bad.")
	}
	return &runResult{
		error:  nil,
		status: "success",
		start:  time.Now(),
	}, nil
}

type runResult struct {
	error
	status string
	start  time.Time
}

func (r *runResult) Status() string       { return r.status }
func (r *runResult) StartTime() time.Time { return r.start }
