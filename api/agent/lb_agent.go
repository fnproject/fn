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

type AckSyncResponseWriter struct {
	http.ResponseWriter
	origin http.ResponseWriter
	acked  chan struct{}
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

	errPlace := make(chan error, 1)

	call.w = &AckSyncResponseWriter{
		origin: call.ResponseWriter(),
		acked:  make(chan struct{}, 1),
	}
	isAckSync := call.Type == models.TypeAcksync
	rw := call.w.(*AckSyncResponseWriter)
	// change the context if it is a acksync call
	if isAckSync {
		ctx = common.BackgroundContext(ctx)
		// We don't want this to run indefinetely we need to guard this context
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, time.Duration(60+call.Timeout))
		defer cancel()
	}

	go a.spawnPlaceCall(ctx, call, errPlace)

	for {
		select {
		case err := <-errPlace:
			return err
		case <-rw.acked:
			// if it is an acksync we return immediately otherwise we ignore the ack
			if isAckSync {
				return nil
			}
		}
	}
}

func (a *lbAgent) spawnPlaceCall(ctx context.Context, call *call, errCh chan error) {
	err := a.placer.PlaceCall(a.rp, ctx, call)
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

func (w *AckSyncResponseWriter) Heaader() http.Header {
	return w.origin.Header()
}

func (w *AckSyncResponseWriter) Write(data []byte) (int, error) {
	return w.origin.Write(data)
}

func (w *AckSyncResponseWriter) WriteHeader(statusCode int) {
	w.origin.WriteHeader(statusCode)
	w.acked <- struct{}{}
}

var _ Agent = &lbAgent{}
var _ callTrigger = &lbAgent{}
