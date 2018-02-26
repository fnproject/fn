package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

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

// The LB agent performs its functionality by delegating to a remote node. It
// pretends to have allocated a slot, and slot.exec() is what actually handles
// the protocol with the remote node; this Slot implementation is used.
type remoteSlot struct {
	lb *lbAgent
}

func (s *remoteSlot) exec(ctx context.Context, call *call) error {
	// TODO: do it properly!
	a := s.lb

	memMb := call.Model().Memory
	lbGroupID := GetGroupID(call.Model())

	capacityRequest := &poolmanager.CapacityEntry{TotalMemoryMb: memMb}
	a.np.AssignCapacity(capacityRequest, lbGroupID)
	defer a.np.ReleaseCapacity(capacityRequest, lbGroupID)

	err := a.placer.PlaceCall(a.np, ctx, call, lbGroupID)
	if err != nil {
		logrus.WithError(err).Error("Failed to place call")
	}
	return err
}

type Placer interface {
	PlaceCall(np NodePool, ctx context.Context, call *call, lbGroupID string) error
}

type naivePlacer struct {
}

func (sp *naivePlacer) PlaceCall(np NodePool, ctx context.Context, call *call, lbGroupID string) error {
	deadline := call.slotDeadline

	for {
		if time.Now().After(deadline) {
			return models.ErrCallTimeoutServerBusy
		}

		// TODO we might need to diff new runner set with previous and explicitly close dropped ones
		runnerList := np.Runners(lbGroupID)

		for _, r := range runnerList {
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

func (s *remoteSlot) Close(ctx context.Context) error {
	return nil
}

func (s *remoteSlot) Error() error {
	return nil
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
}

func NewLBAgent(npmAddress string, agent Agent, cert string, key string, ca string) Agent {
	return &lbAgent{
		delegatedAgent: agent,
		np:             NewgRPCNodePool(npmAddress, cert, key, ca),
		placer:         &naivePlacer{},
	}
}

// GetCall delegates to the wrapped agent, but it adds a "slot reservation" for
// a remoteSlot which will implement the actual running functionality.
func (a *lbAgent) GetCall(opts ...CallOpt) (Call, error) {
	slot := &remoteSlot{lb: a}
	opts = append(opts, WithReservedSlot(context.Background(), slot))
	return a.delegatedAgent.GetCall(opts...)
}

func (a *lbAgent) Close() error {
	a.np.Shutdown()
	return nil
}

func GetGroupID(call *models.Call) string {
	// TODO we need to make LBGroups part of data model so at the moment we just fake it
	// with this dumb method
	return "foobar"
}

func (a *lbAgent) Submit(call Call) error {
	return a.delegatedAgent.Submit(call)

}

func (a *lbAgent) Stats() Stats {
	return a.delegatedAgent.Stats()
}

func (a *lbAgent) PromHandler() http.Handler {
	return a.delegatedAgent.PromHandler()
}

func (a *lbAgent) AddCallListener(cl fnext.CallListener) {
	a.delegatedAgent.AddCallListener(cl)
}

func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Fatal("Enqueue not implemented. Panicking.")
	return nil
}
