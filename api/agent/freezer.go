package agent

import (
	"context"
	"runtime"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
)

// Linux Only cGroups freezer/pause helper. If no notifications are received for
// a period, when freezer state is freezable, we freeze the container. Similarly,
// if the container is frozen and notifications are received to unfreeze it, we
// unfreeze that container.

const (
	FreezerIdleTimeout = time.Duration(50) * time.Millisecond
)

type freezer struct {
	enabled       bool
	notifications chan bool
	errors        chan error
}

type Freezer interface {
	// Set container freezable state. If an error is returned, the
	// caller must cleanup and terminate. Errors are not recoverable.
	SetFreezable(ctx context.Context, isFreezable bool) error
}

func NewFreezer(ctx context.Context, driver drivers.Driver, container drivers.ContainerTask) Freezer {

	freezeObj := &freezer{
		enabled:       runtime.GOOS == "linux",
		notifications: make(chan bool),
		errors:        make(chan error, 1),
	}

	if !freezeObj.enabled {
		return freezeObj
	}

	go func() {
		isFrozen := false
		isFreezable := true

		defer func() {
			close(freezeObj.errors)
		}()

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
	}()

	return freezeObj
}

func (a *freezer) SetFreezable(ctx context.Context, isFreezable bool) error {
	var err error

	if a.enabled {
		select {
		case a.notifications <- isFreezable:
		case err = <-a.errors:
		case <-ctx.Done():
		}
	}

	return err
}
