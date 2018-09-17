/* The consistent hash ring from the original fnlb.
   The behaviour of this depends on changes to the runner list leaving it relatively stable.
*/
package runnerpool

import (
	"context"

	"github.com/fnproject/fn/api/models"

	"github.com/dchest/siphash"
	"github.com/sirupsen/logrus"
)

type chPlacer struct {
	cfg PlacerConfig
}

func NewCHPlacer(cfg *PlacerConfig) Placer {
	logrus.Infof("Creating new CH runnerpool placer with config=%+v", cfg)
	return &chPlacer{
		cfg: *cfg,
	}
}

// This borrows the CH placement algorithm from the original FNLB.
// Because we ask a runner to accept load (queuing on the LB rather than on the nodes), we don't use
// the LB_WAIT to drive placement decisions: runners only accept work if they have the capacity for it.
func (p *chPlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error {

	state := NewPlacerTracker(ctx, &p.cfg)
	defer state.HandleDone()

	key := call.Model().FnID
	sum64 := siphash.Hash(0, 0x4c617279426f6174, []byte(key))

	for {
		runners, err := rp.Runners(call)
		if err != nil {
			state.HandleFindRunnersFailure(err)
			return err
		}

		i := int(jumpConsistentHash(sum64, int32(len(runners))))
		for j := 0; j < len(runners) && !state.IsDone(); j++ {

			r := runners[i]

			placed, err := state.TryRunner(r, call)
			if placed {
				return err
			}

			i = (i + 1) % len(runners)
		}

		if !state.RetryAllBackoff(len(runners)) {
			break
		}
	}

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
