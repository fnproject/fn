package agent

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

func (a *agent) asyncDequeue() {
	a.wg.Add(1)
	defer a.wg.Done() // we can treat this thread like one big task and get safe shutdown fo free

	for {
		select {
		case <-a.shutdown:
			return
		default:
		}

		ctx := context.Background()
		model, err := a.mq.Reserve(ctx)
		if err != nil || model == nil {
			if err != nil {
				logrus.WithError(err).Error("error fetching queued calls")
			}
			time.Sleep(1 * time.Second) // backoff a little
			continue
		}

		callI, err := a.GetCall(FromModel(model))
		if err != nil {
			logrus.WithError(err).Error("error getting async call")
			continue
		}

		call := callI.(*call)
		ch := make(chan slot)
		ctx, cancel := context.WithTimeout(ctx, 900*time.Second)
		defer cancel()

		// We'll wait up to 900 secs for resources for this async job
		go func() {
			a.wg.Add(1)
			defer a.wg.Done()

			slot, err := a.getSlot(ctx, call)
			if err != nil {
				logrus.WithFields(logrus.Fields{"id": call.Model().ID}).WithError(err).Error("error waiting for slot for async call")
				close(ch)
			} else {
				ch <- slot
			}
		}()

		// Here we wait on either shutdown or slot. If slot channel is closed, then we let this fall through,
		// so that Submit() can attempt to get slot again and propagate the errors/logging consistently.
		select {
		case <-a.shutdown:
			cancel()
			return
		case slot, isOpen := <-ch:
			if isOpen {
				// we got a slot, reserve/set it for this call to be used again by Submit() below
				call.slot = slot
			}
		}

		go func() {
			a.wg.Add(1)       // need to add 1 in this thread to ensure safe shutdown
			defer a.wg.Done() // can shed it after this is done, Submit will add 1 too but it's fine

			// TODO if the task is cold and doesn't require reading STDIN, it could
			// run but we may not listen for output since the task timed out. these
			// are at least once semantics, which is really preferable to at most
			// once, so let's do it for now

			err = a.Submit(call)
			if err != nil {
				// NOTE: these could be errors / timeouts from the call that we're
				// logging here (i.e. not our fault), but it's likely better to log
				// these than suppress them so...
				id := call.Model().ID
				logrus.WithFields(logrus.Fields{"id": id}).WithError(err).Error("error running async call")
			}
		}()
	}
}
