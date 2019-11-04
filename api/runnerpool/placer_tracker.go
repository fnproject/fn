package runnerpool

import (
	"context"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"

	"go.opencensus.io/stats"
)

type placerTracker struct {
	cfg        *PlacerConfig
	requestCtx context.Context
	placerCtx  context.Context
	cancel     context.CancelFunc
	tracker    *attemptTracker
	isPlaced   bool
}

func NewPlacerTracker(requestCtx context.Context, cfg *PlacerConfig, call RunnerCall) *placerTracker {

	timeout := cfg.PlacerTimeout
	if call.Model().Type == models.TypeDetached {
		timeout = cfg.DetachedPlacerTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	return &placerTracker{
		cfg:        cfg,
		requestCtx: requestCtx,
		placerCtx:  ctx,
		cancel:     cancel,
		tracker:    newAttemptTracker(requestCtx),
	}
}

// IsDone is a non-blocking check to see if the underlying deadlines are exceeded.
func (tr *placerTracker) IsDone() bool {
	return tr.requestCtx.Err() != nil || tr.placerCtx.Err() != nil
}

// HandleFindRunnersFailure is a convenience function to record error from runnerpool.Runners()
func (tr *placerTracker) HandleFindRunnersFailure(err error) {
	logger := common.Logger(tr.requestCtx).WithError(err)
	w, ok := err.(models.APIErrorWrapper)
	if ok {
		logger = logger.WithField("root_error", w.RootError())
	}
	logger.Warn("Failed to find runners for call")
	stats.Record(tr.requestCtx, errorPoolCountMeasure.M(0))
}

// TryRunner is a convenience function to TryExec a call on a runner and
// analyze the results.
func (tr *placerTracker) TryRunner(r Runner, call RunnerCall) (bool, error) {
	tr.tracker.recordAttempt()

	// WARNING: Do not use placerCtx here to let requestCtx take its time
	// during container execution.
	ctx, cancel := context.WithCancel(tr.requestCtx)
	isPlaced, err := r.TryExec(ctx, call)
	cancel()

	if !isPlaced {

		// Too Busy is super common case, we track it separately
		if err == models.ErrCallTimeoutServerBusy {
			stats.Record(tr.requestCtx, retryTooBusyCountMeasure.M(0))
		} else if tr.requestCtx.Err() != err {
			// only record retry due to an error if client did not abort/cancel/timeout
			stats.Record(tr.requestCtx, retryErrorCountMeasure.M(0))
		}

	} else {
		if err == nil {
			stats.Record(tr.requestCtx, placedOKCountMeasure.M(0))
		} else if tr.requestCtx.Err() == err {
			stats.Record(tr.requestCtx, placedAbortCountMeasure.M(0))
		} else {
			stats.Record(tr.requestCtx, placedErrorCountMeasure.M(0))
		}

		// Call is now committed. In other words, it was 'run'. We are done.
		tr.isPlaced = true
	}

	return isPlaced, err
}

// HandleDone is cleanup function to cancel pending contexts and to
// record stats for the placement session.
func (tr *placerTracker) HandleDone() {

	// Cancel Exit Path / Client cancelled/timedout
	if tr.requestCtx.Err() == context.Canceled {
		stats.Record(tr.requestCtx, cancelCountMeasure.M(0))
	} else if tr.requestCtx.Err() == context.DeadlineExceeded {
		stats.Record(tr.requestCtx, timeoutCountMeasure.M(0))
	}

	// This means our placer timed out. We ignore tr.isPlaced calls
	// since we do not check/track placer ctx timeout if a call was
	// actually ran on a runner. This means, placer timeout can be
	// 10 secs, but a call can execute for 60 secs in a container.
	if !tr.isPlaced && tr.placerCtx.Err() != nil {
		stats.Record(tr.requestCtx, placerTimeoutMeasure.M(0))
	}

	tr.tracker.finalizeAttempts(tr.isPlaced)
	tr.cancel()
}

// RetryAllBackoff blocks until it is time to try the runner list again. Returns
// false if the placer should stop trying.
func (tr *placerTracker) RetryAllBackoff(numOfRunners int, err error) bool {
	// This means Placer is operating on an empty list. No runners
	// available. Record it. ALWAYS do this before failing fast.
	if numOfRunners == 0 {
		stats.Record(tr.requestCtx, emptyPoolCountMeasure.M(0))
	}

	// If there are no runners and last call to provision runners failed due
	// to a user error (or misconfiguration) then fail fast.
	if numOfRunners == 0 && err != nil {
		// IsFuncError currently synonymous with tag: 'blame == user'
		// See: runner_fninvoke.handleFnInvokeCall2
		if models.IsFuncError(err) {
			// We also sanity check for a 502 before returning.
			w, ok := err.(models.APIError)
			if ok {
				if 502 == w.Code() {
					return false
				}
			}
		}
	}

	t := common.NewTimer(tr.cfg.RetryAllDelay)
	defer t.Stop()

	select {
	case <-tr.requestCtx.Done(): // client side timeout/cancel
		return false
	case <-tr.placerCtx.Done(): // placer wait timeout
		return false
	case <-t.C:
	}

	return true
}
