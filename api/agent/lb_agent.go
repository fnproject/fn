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

	capacityRequest := &poolmanager.CapacityRequest{TotalMemoryMb: memMb, LBGroupID: lbGroupID}
	a.np.AssignCapacity(capacityRequest)
	defer a.np.ReleaseCapacity(capacityRequest)

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

func (a *lbAgent) Submit(call Call) error {
	return a.delegatedAgent.Submit(call)
}

func (a *lbAgent) AddCallListener(cl fnext.CallListener) {
	a.delegatedAgent.AddCallListener(cl)
}

func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Fatal("Enqueue not implemented. Panicking.")
	return nil
}
