// Package mock provides a fake Driver implementation that is only used for testing.
package mock

import (
	"context"
	"fmt"

	"github.com/fnproject/fn/api/agent/drivers"
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

func (m *Mocker) PrepareCookie(context.Context, drivers.Cookie) error {
	return nil
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

func (c *cookie) Close(context.Context) error { return nil }

func (c *cookie) ContainerOptions() interface{} { return nil }

func (c *cookie) Run(ctx context.Context) error {
	c.m.count++
	if c.m.count%100 == 0 {
		return fmt.Errorf("Mocker error! Bad.")
	}
	return nil
}

var _ drivers.Cookie = &cookie{}
