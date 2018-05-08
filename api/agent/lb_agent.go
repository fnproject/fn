package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"

	"github.com/fnproject/cloudevent"
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
	placementTimeout = 15 * time.Second
)

type lbAgent struct {
	cfg           AgentConfig
	da            DataAccess
	callListeners []fnext.CallListener
	rp            pool.RunnerPool
	placer        pool.Placer

	shutWg       *common.WaitGroup
	callEndCount int64
}

// NewLBAgent creates an Agent that knows how to load-balance function calls
// across a group of runner nodes.
func NewLBAgent(da DataAccess, rp pool.RunnerPool, p pool.Placer) (Agent, error) {

	// TODO: Move the constants above to Agent Config or an LB specific LBAgentConfig
	cfg, err := NewAgentConfig()
	if err != nil {
		logrus.WithError(err).Fatalf("error in lb-agent config cfg=%+v", cfg)
	}
	logrus.Infof("lb-agent starting cfg=%+v", cfg)

	a := &lbAgent{
		cfg:    *cfg,
		da:     da,
		rp:     rp,
		placer: p,
		shutWg: common.NewWaitGroup(),
	}
	return a, nil
}

func (a *lbAgent) Handle(ctx context.Context, event cloudevent.CloudEvent) (cloudevent.CloudEvent, error) {
	// TODO
	return cloudevent.CloudEvent{}, errors.New("Handle not implemented")
}

func (a *lbAgent) AddCallListener(listener fnext.CallListener) {
	a.callListeners = append(a.callListeners, listener)
}

func (a *lbAgent) fireBeforeCall(ctx context.Context, call *models.Call) error {
	return fireBeforeCallFun(a.callListeners, ctx, call)
}

func (a *lbAgent) fireAfterCall(ctx context.Context, call *models.Call) error {
	return fireAfterCallFun(a.callListeners, ctx, call)
}

// GetAppID is to get the match of an app name to its ID
func (a *lbAgent) GetAppID(ctx context.Context, appName string) (string, error) {
	return a.da.GetAppID(ctx, appName)
}

// GetAppByID is to get the app by ID
func (a *lbAgent) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	return a.da.GetAppByID(ctx, appID)
}

func (a *lbAgent) GetRoute(ctx context.Context, appID string, path string) (*models.Route, error) {
	return a.da.GetRoute(ctx, appID, path)
}

func (a *lbAgent) GetCall(opts ...CallOpt) (Call, error) {
	var c call

	for _, o := range opts {
		err := o(&c)
		if err != nil {
			return nil, err
		}
	}

	// TODO typed errors to test
	if c.req == nil || c.Call == nil {
		return nil, errors.New("no model or request provided for call")
	}

	c.da = a.da
	c.ct = a
	c.stderr = &nullReadWriter{}

	ctx, _ := common.LoggerWithFields(c.req.Context(),
		logrus.Fields{"id": c.ID, "app_id": c.AppID, "route": c.Path})
	c.req = c.req.WithContext(ctx)

	c.lbDeadline = time.Now().Add(time.Duration(c.Call.Timeout) * time.Second)

	return &c, nil
}

func (a *lbAgent) Close() error {

	// start closing the front gate first
	ch := a.shutWg.CloseGroupNB()

	// finally shutdown the runner pool
	err := a.rp.Shutdown(context.Background())
	if err != nil {
		logrus.WithError(err).Warn("Runner pool shutdown error")
	}

	// gate-on front-gate, should be completed if delegated agent & runner pool is gone.
	<-ch
	return err
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
	if !a.shutWg.AddSession(1) {
		return models.ErrCallTimeoutServerBusy
	}

	call := callI.(*call)

	ctx, cancel := context.WithDeadline(call.req.Context(), call.lbDeadline)
	call.req = call.req.WithContext(ctx)
	defer cancel()

	ctx, span := trace.StartSpan(ctx, "agent_submit")
	defer span.End()

	statsEnqueue(ctx)

	// first check any excess case of call.End() stacking.
	if atomic.LoadInt64(&a.callEndCount) >= int64(a.cfg.MaxCallEndStacking) {
		a.handleCallEnd(ctx, call, context.DeadlineExceeded, false)
	}

	err := call.Start(ctx)
	if err != nil {
		return a.handleCallEnd(ctx, call, err, false)
	}

	statsDequeueAndStart(ctx)

	// WARNING: isStarted (handleCallEnd) semantics
	// need some consideration here. Similar to runner/agent
	// we consider isCommitted true if call.Start() succeeds.
	// isStarted=true means we will call Call.End().
	err = a.placer.PlaceCall(a.rp, ctx, call)
	if err != nil {
		logrus.WithError(err).Error("Failed to place call")
	}

	return a.handleCallEnd(ctx, call, err, true)
}

func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Fatal("Enqueue not implemented. Panicking.")
	return nil
}

func (a *lbAgent) scheduleCallEnd(fn func()) {
	atomic.AddInt64(&a.callEndCount, 1)
	go func() {
		fn()
		atomic.AddInt64(&a.callEndCount, -1)
		a.shutWg.DoneSession()
	}()
}

func (a *lbAgent) handleCallEnd(ctx context.Context, call *call, err error, isStarted bool) error {
	if isStarted {
		a.scheduleCallEnd(func() {
			ctx = common.BackgroundContext(ctx)
			ctx, cancel := context.WithTimeout(ctx, a.cfg.CallEndTimeout)
			call.End(ctx, err)
			cancel()
		})

		handleStatsEnd(ctx, err)
		return transformTimeout(err, false)
	}

	a.shutWg.DoneSession()
	handleStatsDequeue(ctx, err)
	return transformTimeout(err, true)
}
