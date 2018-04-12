package agent

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/fnext"
)

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
	shutWg         *common.WaitGroup
}

// NewLBAgent creates an Agent that knows how to load-balance function calls
// across a group of runner nodes.
func NewLBAgent(da DataAccess, rp pool.RunnerPool, p pool.Placer) (Agent, error) {
	agent := createAgent(da, false)
	a := &lbAgent{
		delegatedAgent: agent,
		rp:             rp,
		placer:         p,
		shutWg:         common.NewWaitGroup(),
	}
	return a, nil
}

// GetAppID is to get the match of an app name to its ID
func (a *lbAgent) GetAppID(ctx context.Context, appName string) (string, error) {
	return a.delegatedAgent.GetAppID(ctx, appName)
}

// GetAppByID is to get the app by ID
func (a *lbAgent) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	return a.delegatedAgent.GetAppByID(ctx, appID)
}

// GetCall delegates to the wrapped agent but disables the capacity check as
// this agent isn't actually running the call.
func (a *lbAgent) GetCall(opts ...CallOpt) (Call, error) {
	opts = append(opts, WithoutPreemptiveCapacityCheck())
	return a.delegatedAgent.GetCall(opts...)
}

func (a *lbAgent) Close() error {

	// start closing the front gate first
	ch := a.shutWg.CloseGroupNB()

	// delegated agent shutdown next, blocks here...
	err1 := a.delegatedAgent.Close()
	if err1 != nil {
		logrus.WithError(err1).Warn("Delegated agent shutdown error")
	}

	// finally shutdown the runner pool
	ctx, cancel := context.WithTimeout(context.Background(), runnerPoolShutdownTimeout)
	defer cancel()
	err2 := a.rp.Shutdown(ctx)
	if err2 != nil {
		logrus.WithError(err2).Warn("Runner pool shutdown error")
	}

	// gate-on front-gate, should be completed if delegated agent & runner pool is gone.
	<-ch

	if err1 != nil {
		return err1
	}
	return err2
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
	// we allocate 2 groups one for Submit and one for CallEnd (See HandleCallEnd)
	if !a.shutWg.AddSession(2) {
		return models.ErrCallTimeoutServerBusy
	}
	defer a.shutWg.AddSession(-1)

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

	err := call.Start(ctx)
	if err != nil {
		return a.handleCallEnd(ctx, call, err, false)
	}

	statsDequeueAndStart(ctx)

	err = a.placer.PlaceCall(a.rp, ctx, call)
	if err != nil {
		logrus.WithError(err).Error("Failed to place call")
	}

	return a.handleCallEnd(ctx, call, err, true)
}

func (a *lbAgent) AddCallListener(cl fnext.CallListener) {
	a.delegatedAgent.AddCallListener(cl)
}

func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Fatal("Enqueue not implemented. Panicking.")
	return nil
}

func (a *lbAgent) handleCallEnd(ctx context.Context, call *call, err error, isCommitted bool) error {
	delegatedAgent := a.delegatedAgent.(*agent)
	return delegatedAgent.handleCallEnd(ctx, call, nil, err, isCommitted)
}
