package runnerpool

import (
	"context"
	"time"

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

func NewPlacerTracker(requestCtx context.Context, cfg *PlacerConfig) *placerTracker {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.PlacerTimeout)
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
	common.Logger(tr.requestCtx).WithError(err).Error("Failed to find runners for call")
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
		} else {
			stats.Record(tr.requestCtx, retryErrorCountMeasure.M(0))
		}

	} else {

		// Only log unusual (except for too-busy) errors for isPlaced (customer impacting) calls
		if err != nil && err != models.ErrCallTimeoutServerBusy {
			logger := common.Logger(ctx).WithField("runner_addr", r.Address())
			logger.WithError(err).Errorf("Failed during call placement")
		}

		if err != nil {
			stats.Record(tr.requestCtx, placedErrorCountMeasure.M(0))
		} else {
			stats.Record(tr.requestCtx, placedOKCountMeasure.M(0))
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
	if tr.requestCtx.Err() != nil {
		stats.Record(tr.requestCtx, cancelCountMeasure.M(0))
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
func (tr *placerTracker) RetryAllBackoff(numOfRunners int) bool {

	// This means Placer is operating on an empty list. No runners
	// available. Record it.
	if numOfRunners == 0 {
		stats.Record(tr.requestCtx, emptyPoolCountMeasure.M(0))
	}

	select {
	case <-tr.requestCtx.Done(): // client side timeout/cancel
		return false
	case <-tr.placerCtx.Done(): // placer wait timeout
		return false
	case <-time.After(tr.cfg.RetryAllDelay):
	}

	return true
}
