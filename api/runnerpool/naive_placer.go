package runnerpool

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/models"

	"github.com/sirupsen/logrus"
)

type naivePlacer struct {
	cfg PlacerConfig
}

func NewNaivePlacer(cfg *PlacerConfig) Placer {
	logrus.Infof("Creating new naive runnerpool placer with config=%+v", cfg)
	return &naivePlacer{
		cfg: *cfg,
	}
}

func (sp *naivePlacer) GetPlacerConfig() PlacerConfig {
	return sp.cfg
}

func (sp *naivePlacer) PlaceCall(ctx context.Context, rp RunnerPool, call RunnerCall) error {
	state := NewPlacerTracker(ctx, &sp.cfg, call)
	defer state.HandleDone()

	var runnerPoolErr error
	for {
		var runners []Runner
		runners, runnerPoolErr = rp.Runners(ctx, call)

		rrIndex := uint64(time.Now().Nanosecond())

		for j := 0; j < len(runners) && !state.IsDone(); j++ {

			rrIndex += 1
			r := runners[rrIndex%uint64(len(runners))]

			placed, err := state.TryRunner(r, call)
			if placed {
				return err
			}
		}

		if !state.RetryAllBackoff(len(runners), runnerPoolErr) {
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
