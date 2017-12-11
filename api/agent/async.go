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
		case <-a.resources.WaitAsyncResource():
			// TODO we _could_ return a token here to reserve the ram so that there's
			// not a race between here and Submit but we're single threaded
			// dequeueing and retries handled gracefully inside of Submit if we run
			// out of RAM so..
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // TODO ???
		model, err := a.da.Dequeue(ctx)
		cancel()
		if err != nil || model == nil {
			if err != nil {
				logrus.WithError(err).Error("error fetching queued calls")
			}
			time.Sleep(1 * time.Second) // backoff a little
			continue
		}

		// TODO output / logger should be here too...

		a.wg.Add(1) // need to add 1 in this thread to ensure safe shutdown
		go func() {
			defer a.wg.Done() // can shed it after this is done, Submit will add 1 too but it's fine

			call, err := a.GetCall(FromModel(model))
			if err != nil {
				logrus.WithError(err).Error("error getting async call")
				return
			}

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
