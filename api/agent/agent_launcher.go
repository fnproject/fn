package agent

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/models"
)

// Hot Container launch logic. Below functions determine under
// which conditions to wait or launch a new container. If the system
// is resource starved, evictions are performed.

// implements Slot
type errSlot struct {
	err error
}

func (s *errSlot) Error() error                               { return s.err }
func (s *errSlot) Close() error                               { return nil }
func (s *errSlot) exec(ctx context.Context, call *call) error { panic("bug") }

// tryGetToken attempts to fetch/acquire a token from slot queue without blocking.
func tryGetToken(ch chan Slot) Slot {
	select {
	case s := <-ch:
		return s
	default:
		return nil
	}
}

// waitGetToken blocks and waits on token channel and all waitChans
func waitGetToken(ch chan Slot, waitChans []chan struct{}) Slot {
	for _, wait := range waitChans {
		select {
		case s := <-ch:
			return s
		case <-wait:
		}
	}

	// This is something we do not want to see, but the slight delay
	// below allows slotQueue, etc. to sync up with us. Otherwise,
	// our main loop is too fast for tokens, etc. to show up in slot chan.
	// If 1 msec is not enough for some systems, then failure of this
	// fragile timing may result into more than one container launched.
	// With evictor this should not be a big problem.
	select {
	case s := <-ch:
		return s
	case <-time.After(1 * time.Millisecond):
	}

	return nil
}

// multiplexSlotChan listens [context, agent shutdown, slot channel] and pushes the result to one channel
func (a *agent) multiplexSlotChan(ctx context.Context, call *call, slotChan chan *slotToken) chan Slot {
	output := make(chan Slot)
	dampen := make(chan struct{}, 1)

	go func() {
		dampen <- struct{}{}
		for {
			select {
			case s := <-slotChan:
				if call.slots.acquireSlot(s) {
					select {
					case output <- s.slot:
					case <-ctx.Done():
						s.slot.Close()
					case <-a.shutWg.Closer():
						s.slot.Close()
					}
					return
				}
			case <-ctx.Done():
				output <- &errSlot{ctx.Err()}
				return
			case <-a.shutWg.Closer(): // server shutdown
				output <- &errSlot{models.ErrCallTimeoutServerBusy}
				return
			}
		}
	}()

	<-dampen
	return output
}

// checkLaunch monitors both slot queue and resource tracker to get a token or a new
// container whichever is faster. If a new container is launched, checkLaunch will wait
// until launch completes before trying to spawn more containers.
func (a *agent) checkLaunch(ctx context.Context, call *call, slotChan chan *slotToken) Slot {

	mem := call.Memory + uint64(call.TmpFsSize)
	waitChan := a.multiplexSlotChan(ctx, call, slotChan)

	for {
		// IMPORTANT: Remember if multiple channels are I/O pending, then select
		// will pick at random. Extra tryGetToken() here prioritizes getting Slot over
		// cpu/mem resource if both are available. If we spawned a container in previous
		// iteration, there's no success guarantee here, since it is possible that
		// another request stole that Slot.
		s := tryGetToken(waitChan)
		if s != nil {
			return s
		}

		// Prepare context/cancel for GetResourceToken()
		ctx, cancel := context.WithCancel(ctx)

		// Now wait for [cpu/memory, hot-poll timeout, slot] whichever becomes available
		// first. In case of hot poll timeout, we try to evict a container and repeat the
		// master for-loop. If we get cpu/mem resource, then we proceed to launch a new
		// container.
		select {
		case s := <-waitChan:
			cancel()
			return s
		case resource := <-a.resources.GetResourceToken(ctx, mem, call.CPUs):
			cancel()
			launchChans := make([]chan struct{}, 0, 1)
			launchChans = append(launchChans, make(chan struct{}, 1))
			if !a.launchHot(ctx, call, resource, launchChans[0]) {
				return &errSlot{models.ErrCallTimeoutServerBusy}
			}
			// let's wait for the launch process.
			s := waitGetToken(waitChan, launchChans)
			if s != nil {
				return s
			}
		case <-time.After(a.cfg.HotPoll):
			cancel()
			evictChans := a.evictor.PerformEviction(call.slotHashId, mem, uint64(call.CPUs))
			if len(evictChans) > 0 {
				// let's wait for the evict process.
				s := waitGetToken(waitChan, evictChans)
				if s != nil {
					return s
				}
			} else if a.cfg.EnableNBResourceTracker {
				return &errSlot{models.ErrCallTimeoutServerBusy}
			}
		}
	}
}

func (a *agent) launchHot(ctx context.Context, call *call, tok ResourceToken, ready chan struct{}) bool {
	// If we can't add a session, we are shutting down, return too-busy
	if !a.shutWg.AddSession(1) {
		tok.Close()
		return false
	}

	// get another reference to our slot queue. This ensures during lifetime
	// of the hot container slot queue is not deleted.
	refSlotQueue := a.slotMgr.allocSlotQueue(call.slotHashId)

	// NOTE: runHot will not inherit the timeout from ctx (ignore timings)
	go func() {
		a.runHot(ctx, call, tok, ready)
		a.slotMgr.freeSlotQueue(refSlotQueue)
		a.shutWg.DoneSession()
	}()

	return true
}
