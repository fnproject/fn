// Package mock provides a fake Driver implementation that is only used for testing.
package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
)

func New() drivers.Driver {
	return &Mocker{}
}

type Mocker struct {
	count int
}

func (m *Mocker) CreateCookie(context.Context, drivers.ContainerTask) (drivers.Cookie, error) {
	return &cookie{m}, nil
}

func (m *Mocker) SetPullImageRetryPolicy(policy common.BackOffConfig, checker drivers.RetryErrorChecker) error {
	return nil
}

func (m *Mocker) GetSlotKeyExtensions(extn map[string]string) string {
	return ""
}

func (m *Mocker) Close() error {
	return nil
}

var _ drivers.Driver = &Mocker{}

type cookie struct {
	m *Mocker
}

func (c *cookie) Freeze(context.Context) error {
	return nil
}

func (c *cookie) Unfreeze(context.Context) error {
	return nil
}

func (c *cookie) ValidateImage(context.Context) (bool, error) {
	return false, nil
}

func (c *cookie) PullImage(context.Context) drivers.PullResult {
	return drivers.PullResult{Err: nil, Retries: 0, Duration: 0 * time.Second}
}

func (c *cookie) CreateContainer(context.Context) error {
	return nil
}

func (c *cookie) Close(context.Context) error {
	return nil
}

func (c *cookie) ContainerOptions() interface{} {
	return nil
}

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

var _ drivers.Cookie = &cookie{}

type runResult struct {
	err    error
	status string
	start  time.Time
}

func (r *runResult) Wait(context.Context) drivers.RunResult { return r }
func (r *runResult) Status() string                         { return r.status }
func (r *runResult) Error() error                           { return r.err }
