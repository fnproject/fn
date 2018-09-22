package agent

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/models"
)

type WaitResult struct {
	Slot Slot
	Err  error
}

// tryGetToken attempts to fetch/acquire a token from slot queue without blocking.
func tryGetToken(ch chan WaitResult) (Slot, error) {
	select {
	case s := <-ch:
		return s.Slot, s.Err
	default:
		return nil, nil
	}
}

// waitGetToken blocks and waits on both wait and token channels.
func waitGetToken(ch chan WaitResult, wait chan struct{}) (Slot, error) {
	select {
	case s := <-ch:
		return s.Slot, s.Err
	case <-wait:
	}
	return nil, nil
}

// listenWaitResult listens [context, agent shutdown, slot channel] and pushes the result to one channel
func (a *agent) listenWaitResult(ctx context.Context, call *call, slotChan chan *slotToken) chan WaitResult {
	output := make(chan WaitResult)
	dampen := make(chan struct{}, 1)

	go func() {
		dampen <- struct{}{}
		for {
			select {
			case s := <-slotChan:
				if call.slots.acquireSlot(s) {
					if s.slot.Error() != nil {
						s.slot.Close()
						output <- WaitResult{nil, s.slot.Error()}
						return
					}
					select {
					case output <- WaitResult{s.slot, nil}:
					case <-ctx.Done():
						s.slot.Close()
					case <-a.shutWg.Closer():
						s.slot.Close()
					}
					return
				}
			case <-ctx.Done():
				output <- WaitResult{nil, ctx.Err()}
				return
			case <-a.shutWg.Closer(): // server shutdown
				output <- WaitResult{nil, models.ErrCallTimeoutServerBusy}
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
func (a *agent) checkLaunch(ctx context.Context, call *call, slotChan chan *slotToken) (Slot, error) {

	isAsync := call.Type == models.TypeAsync
	mem := call.Memory + uint64(call.TmpFsSize)
	waitChan := a.listenWaitResult(ctx, call, slotChan)

	// Initial quick check to drain any easy tokens in slot queue. Very happy case.
	s, err := tryGetToken(waitChan)
	if s != nil || err != nil {
		return s, err
	}

	for {
		// IMPORTANT: Remember if multiple channels are I/O pending, then select
		// will pick at random. Extra tryGetToken() here prioritizes getting Slot over
		// cpu/mem resource if both are available. If we spawned a container in previous
		// iteration, there's no success guarantee here, since it is possible that
		// another request stole that Slot.
		s, err := tryGetToken(waitChan)
		if s != nil || err != nil {
			return s, err
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
			return s.Slot, s.Err
		case resource := <-a.resources.GetResourceToken(ctx, mem, call.CPUs, isAsync):
			cancel()
			launchWait := make(chan struct{}, 1)
			if !a.launchHot(ctx, call, resource, launchWait) {
				return nil, models.ErrCallTimeoutServerBusy
			}
			// let's wait for the launch process.
			s, err := waitGetToken(waitChan, launchWait)
			if s != nil || err != nil {
				return s, err
			}
		case <-time.After(a.cfg.HotPoll):
			cancel()
			if !a.evictor.PerformEviction(call.slotHashId, mem, uint64(call.CPUs)) && a.cfg.EnableNBResourceTracker {
				return nil, models.ErrCallTimeoutServerBusy
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
