package agent

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

func (a *agent) asyncDequeue() {
	defer a.wg.Done() // we can treat this thread like one big task and get safe shutdown fo free

	// this is just so we can hang up the dequeue request if we get shut down
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// parent span here so that we can see how many async calls are running
	ctx, span := trace.StartSpan(ctx, "agent_async_dequeue")
	defer span.End()

	for {
		select {
		case <-a.shutdown:
			return
		case <-a.resources.WaitAsyncResource(ctx):
			// TODO we _could_ return a token here to reserve the ram so that there's
			// not a race between here and Submit but we're single threaded
			// dequeueing and retries handled gracefully inside of Submit if we run
			// out of RAM so..
		}

		// we think we can get a cookie now, so go get a cookie
		select {
		case <-a.shutdown:
			return
		case model, ok := <-a.asyncChew(ctx):
			if ok {
				a.wg.Add(1) // need to add 1 in this thread to ensure safe shutdown
				go func(model *models.Call) {
					a.asyncRun(ctx, model)
					a.wg.Done() // can shed it after this is done, Submit will add 1 too but it's fine
				}(model)
			}
		}
	}
}

func (a *agent) asyncChew(ctx context.Context) <-chan *models.Call {
	ch := make(chan *models.Call, 1)

	go func() {
		ctx, cancel := context.WithTimeout(ctx, a.cfg.AsyncChewPoll)
		defer cancel()

		call, err := a.da.Dequeue(ctx)
		if call != nil {
			ch <- call
		} else { // call is nil / error
			if err != nil && err != context.DeadlineExceeded {
				logrus.WithError(err).Error("error fetching queued calls")
			}
			// queue may be empty / unavailable
			time.Sleep(1 * time.Second) // backoff a little before sending no cookie message
			close(ch)
		}
	}()

	return ch
}

func (a *agent) asyncRun(ctx context.Context, model *models.Call) {
	// IMPORTANT: get a context that has a child span but NO timeout (Submit imposes timeout)
	// TODO this is a 'FollowsFrom'
	ctx = common.BackgroundContext(ctx)

	// since async doesn't come in through the normal request path,
	// we've gotta add tags here for stats to come out properly.
	appKey, err := tag.NewKey("fn_appname")
	if err != nil {
		logrus.Fatal(err)
	}
	pathKey, err := tag.NewKey("fn_path")
	if err != nil {
		logrus.Fatal(err)
	}
	ctx, err = tag.New(ctx,
		tag.Insert(appKey, model.AppID),
		tag.Insert(pathKey, model.Path),
	)
	if err != nil {
		logrus.Fatal(err)
	}

	// additional enclosing context here since this isn't spawned from an http request
	ctx, span := trace.StartSpan(ctx, "agent_async_run")
	defer span.End()

	call, err := a.GetCall(
		FromModel(model),
		WithContext(ctx), // NOTE: order is important
	)
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
}
