package agent

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"time"

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
}

type DetachedResponseWriter struct {
	Headers http.Header
	Status  int
	acked   chan struct{}
}

func (w *DetachedResponseWriter) Header() http.Header {
	return w.Headers
}

func (w *DetachedResponseWriter) Write(data []byte) (int, error) {
	return len(data), nil
}

func (w *DetachedResponseWriter) WriteHeader(statusCode int) {
	w.Status = statusCode
	w.acked <- struct{}{}
}

func NewDetachedResponseWriter(h http.Header, statusCode int) *DetachedResponseWriter {
	return &DetachedResponseWriter{
		Headers: h,
		Status:  statusCode,
		acked:   make(chan struct{}, 1),
	}
}

var _ http.ResponseWriter = new(DetachedResponseWriter) // keep the compiler happy

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

	// If overrider is present, let's allow it to modify models.Call
	// and call extensions
	if a.callOverrider != nil {
		ext, err := a.callOverrider(c.Call, c.extensions)
		if err != nil {
			return nil, err
		}
		c.extensions = ext
	}

	setupCtx(&c)

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
func (a *lbAgent) Submit(callI Call) error {
	call := callI.(*call)
	ctx, span := trace.StartSpan(call.req.Context(), "agent_submit")
	defer span.End()

	statsCalls(ctx)

	if !a.shutWg.AddSession(1) {
		statsTooBusy(ctx)
		return models.ErrCallTimeoutServerBusy
	}
	defer a.shutWg.DoneSession()

	statsEnqueue(ctx)

	// pre-read and buffer request body if already not done based
	// on GetBody presence.
	buf, err := a.setRequestBody(ctx, call)
	if buf != nil {
		defer bufPool.Put(buf)
	}
	if err != nil {
		return a.handleCallEnd(ctx, call, err, false)
	}

	err = call.Start(ctx)
	if err != nil {
		return a.handleCallEnd(ctx, call, err, false)
	}

	statsDequeue(ctx)
	statsStartRun(ctx)

	if call.Type == models.TypeDetached {
		return a.placeDetachCall(ctx, call)
	}
	return a.placeCall(ctx, call)
}

func (a *lbAgent) placeDetachCall(ctx context.Context, call *call) error {
	errPlace := make(chan error, 1)
	rw := call.w.(*DetachedResponseWriter)
	go a.spawnPlaceCall(ctx, call, errPlace)
	select {
	case err := <-errPlace:
		return err
	case <-rw.acked:
		return nil
	}
}

func (a *lbAgent) placeCall(ctx context.Context, call *call) error {
	err := a.placer.PlaceCall(a.rp, ctx, call, a.placer.Config().PlacerTimeout)
	return a.handleCallEnd(ctx, call, err, true)
}

func (a *lbAgent) spawnPlaceCall(ctx context.Context, call *call, errCh chan error) {
	var cancel func()
	ctx = common.BackgroundContext(ctx)
	placerTimeout := a.placer.Config().DetachedPlacerTimeout
	// PlacerTimeout for Detached + call.Timeout (inside container)
	ctx, cancel = context.WithTimeout(ctx, placerTimeout+time.Duration(call.Timeout)*time.Second)
	defer cancel()

	err := a.placer.PlaceCall(a.rp, ctx, call, placerTimeout)
	errCh <- a.handleCallEnd(ctx, call, err, true)
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

// implements Agent
func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Error("Enqueue not implemented")
	return errors.New("Enqueue not implemented")
}

func (a *lbAgent) handleCallEnd(ctx context.Context, call *call, err error, isForwarded bool) error {
	if isForwarded {
		call.End(ctx, err)
		statsStopRun(ctx)
		if err == nil {
			statsComplete(ctx)
		}
	} else {
		statsDequeue(ctx)
		if err == context.DeadlineExceeded {
			statsTooBusy(ctx)
			return models.ErrCallTimeoutServerBusy
		}
	}

	if err == models.ErrCallTimeoutServerBusy {
		statsTooBusy(ctx)
		return models.ErrCallTimeoutServerBusy
	} else if err == context.DeadlineExceeded {
		statsTimedout(ctx)
		return models.ErrCallTimeout
	} else if err == context.Canceled {
		statsCanceled(ctx)
	} else if err != nil {
		statsErrors(ctx)
	}
	return err
}

var _ Agent = &lbAgent{}
var _ callTrigger = &lbAgent{}
