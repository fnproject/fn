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

	startTime := time.Now()
	log := common.Logger(ctx)

OutTries:
	for {
		runners, err := rp.Runners(call)
		if err != nil {
			stats.Record(ctx, errorPoolCountMeasure.M(0))
			log.WithError(err).Error("Failed to find runners for call")
		} else {
			if len(runners) == 0 {
				stats.Record(ctx, emptyPoolCountMeasure.M(0))
			}
			for j := 0; j < len(runners); j++ {
				if ctx.Err() != nil {
					break OutTries
				}

				i := atomic.AddUint64(&sp.rrIndex, uint64(1))
				r := runners[int(i)%len(runners)]

				attemptTime := time.Now()
				tryCtx, tryCancel := context.WithCancel(ctx)
				placed, err := r.TryExec(tryCtx, call)
				tryCancel()

				if err != nil && err != models.ErrCallTimeoutServerBusy {
					log.WithError(err).Error("Failed during call placement")
				}
				if placed {
					if err != nil {
						stats.Record(ctx, placedErrorCountMeasure.M(0))
					} else {
						stats.Record(ctx, placedOKCountMeasure.M(0))
					}
					// IMPORTANT: here we use (attempt_time - start_time). We want to exclude TryExec
					// latency *if* TryExec() goes through with success. Placer latency metric only shows
					// how much time are spending in Placer loop/retries. The metric includes rtt/latency of
					// *all* unsuccessful NACK (retriable) responses from runners as well. For example, if
					// Placer loop here retries 4 runners (which takes 5 msecs each) and then 5th runner
					// succeeds (but takes 35 seconds to finish execution), we report 20 msecs as our LB
					// latency.
					stats.Record(ctx, placerLatencyMeasure.M(int64(attemptTime.Sub(startTime)/time.Millisecond)))
					return err
				}

				// Too Busy is super common case, we track it separately
				if err == models.ErrCallTimeoutServerBusy {
					stats.Record(ctx, retryTooBusyCountMeasure.M(0))
				} else {
					stats.Record(ctx, retryErrorCountMeasure.M(0))
				}
			}
		}

		stats.Record(ctx, fullScanCountMeasure.M(0))

		// backoff
		select {
		case <-ctx.Done():
			break OutTries
		case <-time.After(sp.rrInterval):
		}
	}

	// Cancel Exit Path / Client cancelled/timedout
	stats.Record(ctx, cancelCountMeasure.M(0))
	stats.Record(ctx, placerLatencyMeasure.M(int64(time.Now().Sub(startTime)/time.Millisecond)))
	return models.ErrCallTimeoutServerBusy
}
