package runnerpool

import (
	"context"
	"time"
)

type fakeAcksyncPlacer struct {
	cfg       PlacerConfig
	sleeptime time.Duration
}

func NewFakeAcksyncPlacer(cfg *PlacerConfig, st time.Duration) Placer {
	return &fakeAcksyncPlacer{
		cfg:       *cfg,
		sleeptime: st,
	}
}

// PlaceCall for the fakeAcksyncPlacer  just sleeps for a period of time to let the placer context to time out.
// It returns the context exceeded error only if the placer context times out and the request context is still valid
func (p *fakeAcksyncPlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error {
	state := NewPlacerTracker(ctx, &p.cfg, call.Model().Type)
	defer state.HandleDone()
	time.Sleep(p.sleeptime)
	if state.placerCtx.Err() != nil && state.requestCtx.Err() == nil {
		return state.placerCtx.Err()
	}
	return nil
}
