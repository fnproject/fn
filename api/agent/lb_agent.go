package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/fnproject/fn/poolmanager"
)

// RequestReader takes an agent.Call and return a ReadCloser for the request body inside it
func RequestReader(c *Call) (io.ReadCloser, error) {
	// Get the call :(((((
	cc, ok := (*c).(*call)

	if !ok {
		return nil, errors.New("Can't cast agent.Call to agent.call")
	}

	if cc.req == nil {
		return nil, errors.New("Call doesn't contain a request")
	}

	logrus.Info(cc.req)

	return cc.req.Body, nil
}

func ResponseWriter(c *Call) (*http.ResponseWriter, error) {
	cc, ok := (*c).(*call)

	if !ok {
		return nil, errors.New("Can't cast agent.Call to agent.call")
	}

	if rw, ok := cc.w.(http.ResponseWriter); ok {
		return &rw, nil
	}

	return nil, errors.New("Unable to get HTTP response writer from the call")
}

type remoteSlot struct {
	lbAgent *lbAgent
}

func (s *remoteSlot) exec(ctx context.Context, call *call) error {
	a := s.lbAgent

	memMb := call.Model().Memory
	lbGroupID := GetGroupID(call.Model())

	capacityRequest := &poolmanager.CapacityRequest{TotalMemoryMb: memMb, LBGroupID: lbGroupID}
	a.np.AssignCapacity(capacityRequest)
	defer a.np.ReleaseCapacity(capacityRequest)

	err := a.placer.PlaceCall(a.np, ctx, call, lbGroupID)
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

type Placer interface {
	PlaceCall(np NodePool, ctx context.Context, call *call, lbGroupID string) error
}

type naivePlacer struct {
}

func NewNaivePlacer() Placer {
	return &naivePlacer{}
}

func (sp *naivePlacer) PlaceCall(np NodePool, ctx context.Context, call *call, lbGroupID string) error {
	deadline := call.slotDeadline

	for {
		if time.Now().After(deadline) {
			return models.ErrCallTimeoutServerBusy
		}

		for _, r := range np.Runners(lbGroupID) {
			placed, err := r.TryExec(ctx, call)
			if err != nil {
				logrus.WithError(err).Error("Failed during call placement")
			}
			if placed {
				return err
			}

		}

		time.Sleep(retryWaitInterval)
	}

}

const (
	runnerReconnectInterval = 5 * time.Second
	// sleep time to attempt placement across all runners before retrying
	retryWaitInterval = 10 * time.Millisecond
	// sleep time when scaling from 0 to 1 runners
	noCapacityWaitInterval = 1 * time.Second
	// amount of time to wait to place a request on a runner
	placementTimeout = 15 * time.Second
)

type lbAgent struct {
	delegatedAgent Agent
	np             NodePool
	placer         Placer

	wg       sync.WaitGroup // Needs a good name
	shutdown chan struct{}
}

func NewLBAgent(agent Agent, np NodePool, p Placer) (Agent, error) {
	a := &lbAgent{
		delegatedAgent: agent,
		np:             np,
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
	a.np.Shutdown()
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
