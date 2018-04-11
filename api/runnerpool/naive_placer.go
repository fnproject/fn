package runnerpool

import (
	"context"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// sleep time to attempt placement across all runners before retrying
	retryWaitInterval = 10 * time.Millisecond
)

type naivePlacer struct{}

func NewNaivePlacer() Placer {
	logrus.Info("Creating new naive runnerpool placer")
	return &naivePlacer{}
}

func (sp *naivePlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error {
	timeout := time.After(call.SlotDeadline().Sub(time.Now()))

	for {
		select {
		case <-ctx.Done():
			return models.ErrCallTimeoutServerBusy
		case <-timeout:
			return models.ErrCallTimeoutServerBusy
		default:
			runners, err := rp.Runners(call)
			if err != nil {
				logrus.WithError(err).Error("Failed to find runners for call")
			} else {
				for _, r := range runners {
					placed, err := r.TryExec(ctx, call)
					if err != nil {
						logrus.WithError(err).Error("Failed during call placement")
					}
					if placed {
						return err
					}
				}
			}

			remaining := call.SlotDeadline().Sub(time.Now())
			if remaining <= 0 {
				return models.ErrCallTimeoutServerBusy
			}

			// backoff
			select {
			case <-ctx.Done():
				return models.ErrCallTimeoutServerBusy
			case <-timeout:
				return models.ErrCallTimeoutServerBusy
			case <-time.After(common.MinDuration(retryWaitInterval, remaining)):
			}
		}
	}
}
