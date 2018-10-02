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
	if call.Model().Type == models.TypeAcksync {
		sp.cfg = NewPlacerConfig(30)
	}
	state := NewPlacerTracker(ctx, &sp.cfg)
	defer state.HandleDone()

	var runnerPoolErr error
	for {
		var runners []Runner
		runners, runnerPoolErr = rp.Runners(call)

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

	if runnerPoolErr != nil {
		// If we haven't been able to place the function and we got an error
		// from the runner pool, return that error (since we don't have
		// enough runners to handle the current load and the runner pool is
		// having trouble).
		state.HandleFindRunnersFailure(runnerPoolErr)
		return runnerPoolErr
	}
	return models.ErrCallTimeoutServerBusy
}
