package runnerpool

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/models"

	"github.com/fnproject/fn/api/event"
	"github.com/sirupsen/logrus"
)

type naivePlacer struct {
	cfg     PlacerConfig
	rrIndex uint64
}

func NewNaivePlacer(cfg *PlacerConfig) Placer {
	logrus.Infof("Creating new naive runnerpool placer with config=%+v", cfg)
	return &naivePlacer{
		cfg:     *cfg,
		rrIndex: uint64(time.Now().Nanosecond()),
	}
}

func (sp *naivePlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) (*event.Event, error) {

	state := NewPlacerTracker(ctx, &sp.cfg)
	defer state.HandleDone()

	for {
		runners, err := rp.Runners(call)
		if err != nil {
			log.WithError(err).Error("Failed to find runners for call")
			stats.Record(ctx, errorPoolCountMeasure.M(0))
			tracker.finalizeAttempts(false)
			return nil, err
		}

		for j := 0; j < len(runners) && !state.IsDone(); j++ {

			i := atomic.AddUint64(&sp.rrIndex, uint64(1))
			r := runners[int(i)%len(runners)]

			tracker.recordAttempt()
			tryCtx, tryCancel := context.WithCancel(ctx)
			outevt, placed, err := r.TryExec(tryCtx, call)
			tryCancel()

			// Only log unusual (except for too-busy) errors
			if err != nil && err != models.ErrCallTimeoutServerBusy {
				log.WithError(err).Errorf("Failed during call placement, placed=%v", placed)
			}

			if placed {
				if err != nil {
					stats.Record(ctx, placedErrorCountMeasure.M(0))
					tracker.finalizeAttempts(true)
					return nil, err
				}
				stats.Record(ctx, placedOKCountMeasure.M(0))
				return outevt, err
			}
		}

		if !state.RetryAllBackoff(len(runners)) {
			break
		}
	}

	// Cancel Exit Path / Client cancelled/timedout
	stats.Record(ctx, cancelCountMeasure.M(0))
	tracker.finalizeAttempts(false)
	return nil, models.ErrCallTimeoutServerBusy
}
