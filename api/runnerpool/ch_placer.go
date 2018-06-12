/* The consistent hash ring from the original fnlb.
   The behaviour of this depends on changes to the runner list leaving it relatively stable.
*/
package runnerpool

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"

	"github.com/dchest/siphash"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
)

type chPlacer struct {
	rrInterval time.Duration
}

func NewCHPlacer() Placer {
	logrus.Info("Creating new CH runnerpool placer")
	return &chPlacer{
		rrInterval: 10 * time.Millisecond,
	}
}

// This borrows the CH placement algorithm from the original FNLB.
// Because we ask a runner to accept load (queuing on the LB rather than on the nodes), we don't use
// the LB_WAIT to drive placement decisions: runners only accept work if they have the capacity for it.
func (p *chPlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error {

	startTime := time.Now()
	log := common.Logger(ctx)

	// The key is just the path in this case
	key := call.Model().Path
	sum64 := siphash.Hash(0, 0x4c617279426f6174, []byte(key))

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

			i := int(jumpConsistentHash(sum64, int32(len(runners))))

			for j := 0; j < len(runners); j++ {
				if ctx.Err() != nil {
					break OutTries
				}

				r := runners[i]

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

				i = (i + 1) % len(runners)

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
		case <-time.After(p.rrInterval):
		}
	}

	// Cancel Exit Path / Client cancelled/timedout
	stats.Record(ctx, cancelCountMeasure.M(0))
	stats.Record(ctx, placerLatencyMeasure.M(int64(time.Now().Sub(startTime)/time.Millisecond)))
	return models.ErrCallTimeoutServerBusy
}

// A Fast, Minimal Memory, Consistent Hash Algorithm:
// https://arxiv.org/ftp/arxiv/papers/1406/1406.2294.pdf
func jumpConsistentHash(key uint64, num_buckets int32) int32 {
	var b, j int64 = -1, 0
	for j < int64(num_buckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = (b + 1) * int64((1<<31)/(key>>33)+1)
	}
	return int32(b)
}
