package runnerpool

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"

	"github.com/sirupsen/logrus"
)

const (
	// sleep time to attempt placement across all runners before retrying
	retryWaitInterval = 10 * time.Millisecond
)

type naivePlacer struct {
	rrIndex uint64
}

func NewNaivePlacer() Placer {
	rrIndex := uint64(time.Now().Nanosecond())
	logrus.Infof("Creating new naive runnerpool placer rrIndex=%d", rrIndex)
	return &naivePlacer{
		rrIndex: rrIndex,
	}
}

func (sp *naivePlacer) PlaceCall(rp RunnerPool, ctx context.Context, call RunnerCall) error {
	timeout := time.After(call.LbDeadline().Sub(time.Now()))

	for {
		runners, err := rp.Runners(call)
		if err != nil {
			logrus.WithError(err).Error("Failed to find runners for call")
		} else {
			for j := 0; j < len(runners); j++ {

				select {
				case <-ctx.Done():
					return models.ErrCallTimeoutServerBusy
				case <-timeout:
					return models.ErrCallTimeoutServerBusy
				default:
				}

				i := atomic.AddUint64(&sp.rrIndex, uint64(1))
				r := runners[int(i)%len(runners)]

				tryCtx, tryCancel := context.WithCancel(ctx)
				placed, err := r.TryExec(tryCtx, call)
				tryCancel()

				if err != nil {
					logrus.WithError(err).Error("Failed during call placement")
				}
				if placed {
					return err
				}
			}
		}

		remaining := call.LbDeadline().Sub(time.Now())
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
