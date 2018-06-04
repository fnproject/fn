package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"sync/atomic"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"

	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/fnext"
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

func NewLBAgentConfig() (*AgentConfig, error) {
	cfg, err := NewAgentConfig()
	if err != nil {
		return cfg, err
	}
	if cfg.MaxRequestSize == 0 {
		return cfg, errors.New("lb-agent requires MaxRequestSize limit")
	}
	if cfg.MaxResponseSize == 0 {
		return cfg, errors.New("lb-agent requires MaxResponseSize limit")
	}
	return cfg, nil
}

// NewLBAgentWithConfig creates an Agent configured with a supplied AgentConfig
func NewLBAgentWithConfig(da DataAccess, rp pool.RunnerPool, p pool.Placer, cfg *AgentConfig) (Agent, error) {
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

// NewLBAgent creates an Agent that knows how to load-balance function calls
// across a group of runner nodes.
func NewLBAgent(da DataAccess, rp pool.RunnerPool, p pool.Placer) (Agent, error) {

	// TODO: Move the constants above to Agent Config or an LB specific LBAgentConfig
	cfg, err := NewLBAgentConfig()
	if err != nil {
		logrus.WithError(err).Fatalf("error in lb-agent config cfg=%+v", cfg)
	}
	return NewLBAgentWithConfig(da, rp, p, cfg)
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

	err := setMaxBodyLimit(&a.cfg, &c)
	if err != nil {
		return nil, err
	}

	setupCtx(&c)

	c.isLB = true
	c.da = a.da
	c.ct = a
	c.stderr = &nullReadWriter{}
	c.slotHashId = getSlotQueueKey(&c)

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
	ctx, span := trace.StartSpan(call.req.Context(), "agent_submit")
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

	// pre-read and buffer request body if already not done based
	// on GetBody presence.
	buf, err := a.setRequestBody(ctx, call)
	if buf != nil {
		defer bufPool.Put(buf)
	}
	if err != nil {
		logrus.WithError(err).Error("Failed to process call body")
		return a.handleCallEnd(ctx, call, err, true)
	}

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

// setRequestGetBody sets GetBody function on the given http.Request if it is missing.  GetBody allows
// reading from the request body without mutating the state of the request.
func (a *lbAgent) setRequestBody(ctx context.Context, call *call) (*bytes.Buffer, error) {

	r := call.req
	if r.Body == nil || r.GetBody != nil {
		return nil, nil
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()

	// WARNING: we need to handle IO in a separate go-routine below
	// to be able to detect a ctx timeout. When we timeout, we
	// let gin/http-server to unblock the go-routine below.
	errApp := make(chan error, 1)
	go func() {

		_, err := buf.ReadFrom(r.Body)
		if err != nil && err != io.EOF {
			errApp <- err
			return
		}

		r.Body = ioutil.NopCloser(bytes.NewReader(buf.Bytes()))

		// GetBody does not mutate the state of the request body
		r.GetBody = func() (io.ReadCloser, error) {
			return ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
		}

		close(errApp)
	}()

	select {
	case err := <-errApp:
		return buf, err
	case <-ctx.Done():
		return buf, ctx.Err()
	}
}

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
