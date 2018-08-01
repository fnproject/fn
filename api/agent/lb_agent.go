package agent

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/fnext"
)

type lbAgent struct {
	cfg           Config
	cda           CallHandler
	callListeners []fnext.CallListener
	rp            pool.RunnerPool
	placer        pool.Placer
	callOverrider CallOverrider
	shutWg        *common.WaitGroup
	callEndCount  int64
}

type LBAgentOption func(*lbAgent) error

func WithLBAgentConfig(cfg *Config) LBAgentOption {
	return func(a *lbAgent) error {
		a.cfg = *cfg
		return nil
	}
}

// LB agents can use this to register a CallOverrider to modify a Call and extensions
func WithLBCallOverrider(fn CallOverrider) LBAgentOption {
	return func(a *lbAgent) error {
		if a.callOverrider != nil {
			return errors.New("lb-agent call overriders already exists")
		}
		a.callOverrider = fn
		return nil
	}
}

// NewLBAgent creates an Agent that knows how to load-balance function calls
// across a group of runner nodes.
func NewLBAgent(da CallHandler, rp pool.RunnerPool, p pool.Placer, options ...LBAgentOption) (Agent, error) {

	// Yes, LBAgent and Agent both use an Config.
	cfg, err := NewConfig()
	if err != nil {
		logrus.WithError(err).Fatalf("error in lb-agent config cfg=%+v", cfg)
	}

	a := &lbAgent{
		cfg:    *cfg,
		cda:    da,
		rp:     rp,
		placer: p,
		shutWg: common.NewWaitGroup(),
	}

	// Allow overriding config
	for _, option := range options {
		err = option(a)
		if err != nil {
			logrus.WithError(err).Fatalf("error in lb-agent options")
		}
	}

	logrus.Infof("lb-agent starting cfg=%+v", a.cfg)
	return a, nil
}

// implements Agent
func (a *lbAgent) AddCallListener(listener fnext.CallListener) {
	a.callListeners = append(a.callListeners, listener)
}

// implements callTrigger
func (a *lbAgent) fireBeforeCall(ctx context.Context, call *models.Call) error {
	return fireBeforeCallFun(a.callListeners, ctx, call)
}

// implements callTrigger
func (a *lbAgent) fireAfterCall(ctx context.Context, call *models.Call) error {
	return fireAfterCallFun(a.callListeners, ctx, call)
}

// implements Agent
func (a *lbAgent) GetCall(ctx context.Context, opts ...CallOpt) (Call, error) {
	var c call

	for _, o := range opts {
		err := o(ctx, &c)
		if err != nil {
			return nil, err
		}
	}

	// TODO typed errors to test
	if c.Call == nil {
		return nil, errors.New("no model or request provided for call")
	}

	// If overrider is present, let's allow it to modify models.Call
	// and call extensions
	if a.callOverrider != nil {
		ext, err := a.callOverrider(c.Call, c.extensions)
		if err != nil {
			return nil, err
		}
		c.extensions = ext
	}

	c.isLB = true
	c.handler = a.cda
	c.ct = a
	c.stderr = &nullReadWriter{}
	c.slotHashId = getSlotQueueKey(&c)
	return &c, nil
}

// implements Agent
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

// implements Agent
func (a *lbAgent) Submit(ctx context.Context, callI Call) error {
	if !a.shutWg.AddSession(1) {
		return models.ErrCallTimeoutServerBusy
	}

	call := callI.(*call)
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
		common.Logger(ctx).WithError(err).Error("Failed to place call")
	}

	return a.handleCallEnd(ctx, call, err, true)
}

// implements Agent
func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Error("Enqueue not implemented")
	return errors.New("Enqueue not implemented")
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

var _ Agent = &lbAgent{}
var _ callTrigger = &lbAgent{}
