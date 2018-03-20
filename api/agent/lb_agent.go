package agent

import (
	"context"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/fnext"
)

type remoteSlot struct {
	lbAgent *lbAgent
}

func (s *remoteSlot) exec(ctx context.Context, call pool.RunnerCall) error {
	a := s.lbAgent

	err := a.placer.PlaceCall(a.rp, ctx, call)
	if err != nil {
		logrus.WithError(err).Error("Failed to place call")
	}
	return err
}

func (s *remoteSlot) Close(ctx context.Context) error {
	return nil
}

func (s *remoteSlot) Error() error {
	return nil
}

type naivePlacer struct {
}

func NewNaivePlacer() pool.Placer {
	return &naivePlacer{}
}

func minDuration(f, s time.Duration) time.Duration {
	if f < s {
		return f
	}
	return s
}

func (sp *naivePlacer) PlaceCall(rp pool.RunnerPool, ctx context.Context, call pool.RunnerCall) error {
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
			time.Sleep(minDuration(retryWaitInterval, remaining))
		}
	}
}

const (
	runnerReconnectInterval = 5 * time.Second
	// sleep time to attempt placement across all runners before retrying
	retryWaitInterval = 10 * time.Millisecond
	// sleep time when scaling from 0 to 1 runners
	noCapacityWaitInterval = 1 * time.Second
	// amount of time to wait to place a request on a runner
	placementTimeout          = 15 * time.Second
	runnerPoolShutdownTimeout = 5 * time.Second
)

type lbAgent struct {
	delegatedAgent Agent
	rp             pool.RunnerPool
	placer         pool.Placer

	wg       sync.WaitGroup // Needs a good name
	shutdown chan struct{}
}

func NewLBAgent(da DataAccess, rp pool.RunnerPool, p pool.Placer) (Agent, error) {
	agent := createAgent(da, false)
	a := &lbAgent{
		delegatedAgent: agent,
		rp:             rp,
		placer:         p,
	}
	return a, nil
}

// GetCall delegates to the wrapped agent but disables the capacity check as
// this agent isn't actually running the call.
func (a *lbAgent) GetCall(opts ...CallOpt) (Call, error) {
	opts = append(opts, WithoutPreemptiveCapacityCheck())
	return a.delegatedAgent.GetCall(opts...)
}

func (a *lbAgent) Close() error {
	// we should really be passing the server's context here
	ctx, cancel := context.WithTimeout(context.Background(), runnerPoolShutdownTimeout)
	defer cancel()

	a.rp.Shutdown(ctx)
	err := a.delegatedAgent.Close()
	if err != nil {
		return err
	}
	return nil
}

func GetGroupID(call *models.Call) string {
	// TODO until fn supports metadata, allow LB Group ID to
	// be overridden via configuration.
	// Note that employing this mechanism will expose the value of the
	// LB Group ID to the function as an environment variable!
	lbgID := call.Config["FN_LB_GROUP_ID"]
	if lbgID == "" {
		return "default"
	}
	return lbgID
}

func (a *lbAgent) Submit(callI Call) error {
	a.wg.Add(1)
	defer a.wg.Done()

	select {
	case <-a.shutdown:
		return models.ErrCallTimeoutServerBusy
	default:
	}

	call := callI.(*call)

	ctx, cancel := context.WithDeadline(call.req.Context(), call.execDeadline)
	call.req = call.req.WithContext(ctx)
	defer cancel()

	ctx, span := trace.StartSpan(ctx, "agent_submit")
	defer span.End()

	err := a.submit(ctx, call)
	return err
}

func (a *lbAgent) submit(ctx context.Context, call *call) error {
	statsEnqueue(ctx)

	a.startStateTrackers(ctx, call)
	defer a.endStateTrackers(ctx, call)

	slot := &remoteSlot{lbAgent: a}

	defer slot.Close(ctx) // notify our slot is free once we're done

	err := call.Start(ctx)
	if err != nil {
		handleStatsDequeue(ctx, err)
		return transformTimeout(err, true)
	}

	statsDequeueAndStart(ctx)

	// pass this error (nil or otherwise) to end directly, to store status, etc
	err = slot.exec(ctx, call)
	handleStatsEnd(ctx, err)

	// TODO: we need to allocate more time to store the call + logs in case the call timed out,
	// but this could put us over the timeout if the call did not reply yet (need better policy).
	ctx = common.BackgroundContext(ctx)
	err = call.End(ctx, err)
	return transformTimeout(err, false)
}

func (a *lbAgent) AddCallListener(cl fnext.CallListener) {
	a.delegatedAgent.AddCallListener(cl)
}

func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Fatal("Enqueue not implemented. Panicking.")
	return nil
}

func (a *lbAgent) startStateTrackers(ctx context.Context, call *call) {
	delegatedAgent := a.delegatedAgent.(*agent)
	delegatedAgent.startStateTrackers(ctx, call)
}

func (a *lbAgent) endStateTrackers(ctx context.Context, call *call) {
	delegatedAgent := a.delegatedAgent.(*agent)
	delegatedAgent.endStateTrackers(ctx, call)
}
