package runnerpool

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
)

type naivePlacer struct {
	rrInterval time.Duration
	rrIndex    uint64
}

func NewNaivePlacer() Placer {
	rrIndex := uint64(time.Now().Nanosecond())
	logrus.Infof("Creating new naive runnerpool placer rrIndex=%d", rrIndex)
	return &naivePlacer{
		rrInterval: 10 * time.Millisecond,
		rrIndex:    rrIndex,
	}
}

func (sp *naivePlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error {

	tracker := newAttemptTracker(ctx)
	log := common.Logger(ctx)

OutTries:
	for {
		runners, err := rp.Runners(call)
		if err != nil {
			log.WithError(err).Error("Failed to find runners for call")
			stats.Record(ctx, errorPoolCountMeasure.M(0))
			tracker.finalizeAttempts(false)
			return err
		}

		for j := 0; j < len(runners); j++ {
			if ctx.Err() != nil {
				break OutTries
			}

			i := atomic.AddUint64(&sp.rrIndex, uint64(1))
			r := runners[int(i)%len(runners)]

			tracker.recordAttempt()
			tryCtx, tryCancel := context.WithCancel(ctx)
			placed, err := r.TryExec(tryCtx, call)
			tryCancel()

			// Only log unusual (except for too-busy) errors
			if err != nil && err != models.ErrCallTimeoutServerBusy {
				log.WithError(err).Errorf("Failed during call placement, placed=%v", placed)
			}

			if placed {
				if err != nil {
					stats.Record(ctx, placedErrorCountMeasure.M(0))
				} else {
					stats.Record(ctx, placedOKCountMeasure.M(0))
				}
				tracker.finalizeAttempts(true)
				return err
			}

			// Too Busy is super common case, we track it separately
			if err == models.ErrCallTimeoutServerBusy {
				stats.Record(ctx, retryTooBusyCountMeasure.M(0))
			} else {
				stats.Record(ctx, retryErrorCountMeasure.M(0))
			}
		}

		if len(runners) == 0 {
			stats.Record(ctx, emptyPoolCountMeasure.M(0))
		}

		// backoff
		select {
		case <-ctx.Done():
			break OutTries
		case <-time.After(sp.rrInterval):
		}
	}

	// Cancel Exit Path / Client cancelled/timedout
	stats.Record(ctx, cancelCountMeasure.M(0))
	tracker.finalizeAttempts(false)
	return models.ErrCallTimeoutServerBusy
}
