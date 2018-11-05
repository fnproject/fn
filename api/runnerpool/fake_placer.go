package runnerpool

import (
	"context"
	"time"
)

type fakeDetachedPlacer struct {
	cfg       PlacerConfig
	sleeptime time.Duration
}

func NewFakeDetachedPlacer(cfg *PlacerConfig, st time.Duration) Placer {
	return &fakeDetachedPlacer{
		cfg:       *cfg,
		sleeptime: st,
	}
}

func (p *fakeDetachedPlacer) Config() PlacerConfig {
	return p.cfg
}

// PlaceCall for the fakeDetachedPlacer  just sleeps for a period of time to let the placer context to time out.
// It returns the context exceeded error only if the placer context times out and the request context is still valid
func (p *fakeDetachedPlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall, placerTimeout time.Duration) error {
	state := NewPlacerTracker(ctx, &p.cfg, placerTimeout)
	defer state.HandleDone()
	time.Sleep(p.sleeptime)
	if state.placerCtx.Err() != nil && state.requestCtx.Err() == nil {
		return state.placerCtx.Err()
	}
	return nil
}
