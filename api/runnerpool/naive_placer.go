package runnerpool

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/models"

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

func (sp *naivePlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error {

	state := NewPlacerTracker(ctx, &sp.cfg)
	defer state.HandleDone()

	for {
		runners, err := rp.Runners(call)
		if err != nil {
			state.HandleFindRunnersFailure(err)
			return err
		}

		for j := 0; j < len(runners) && !state.IsDone(); j++ {

			i := atomic.AddUint64(&sp.rrIndex, uint64(1))
			r := runners[int(i)%len(runners)]

			placed, err := state.TryRunner(r, call)
			if placed {
				return err
			}
		}

		if !state.RetryAllBackoff(len(runners)) {
			break
		}
	}

	return models.ErrCallTimeoutServerBusy
}
