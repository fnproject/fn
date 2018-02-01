package agent

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
)

const (
	FreezerIdleTimeout = time.Duration(50) * time.Millisecond
)

type freezer struct {
	notifications chan bool
	errors        chan error
}

type Freezer interface {
	SetFreezable(ctx context.Context, isFreezable bool) error
}

func NewFreezer(ctx context.Context, driver drivers.Driver, container drivers.ContainerTask) Freezer {

	freezeObj := &freezer{
		notifications: make(chan bool),
		errors:        make(chan error, 1),
	}

	go func() {
		isFrozen := false
		isFreezable := true

		for {
			if isFrozen {
				select {
				case isFreezable = <-freezeObj.notifications:
					if isFreezable == isFrozen {
						continue
					}
					isFrozen = isFreezable
				case <-ctx.Done():
					return
				}
			} else {
				select {
				case isFreezable = <-freezeObj.notifications:
					continue
				case <-time.After(FreezerIdleTimeout):
					if isFreezable == isFrozen {
						continue
					}
					isFrozen = isFreezable
				case <-ctx.Done():
					return
				}
			}

			var err error
			if isFrozen {
				err = driver.Freeze(ctx, container)
			} else {
				err = driver.Unfreeze(ctx, container)
			}
			if err != nil {
				freezeObj.errors <- err
				return
			}
		}

		close(freezeObj.errors)
	}()

	return freezeObj
}

func (a *freezer) SetFreezable(ctx context.Context, isFreezable bool) error {
	select {
	case a.notifications <- isFreezable:
	case err := <-a.errors:
		return err
	case <-ctx.Done():
	}
	return nil
}
