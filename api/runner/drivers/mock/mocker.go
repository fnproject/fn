package mock

import (
	"context"
	"fmt"

	"gitlab.oracledx.com/odx/functions/api/runner/drivers"
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

func (c *cookie) Close() error { return nil }

func (c *cookie) Run(ctx context.Context) (drivers.RunResult, error) {
	c.m.count++
	if c.m.count%100 == 0 {
		return nil, fmt.Errorf("Mocker error! Bad.")
	}
	return &runResult{
		error:       nil,
		StatusValue: "success",
	}, nil
}

type runResult struct {
	error
	StatusValue string
}

func (runResult *runResult) Status() string {
	return runResult.StatusValue
}
