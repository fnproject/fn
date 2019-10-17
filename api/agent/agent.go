package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	dockerdriver "github.com/fnproject/fn/api/agent/drivers/docker"
	driver_stats "github.com/fnproject/fn/api/agent/drivers/stats"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/fsnotify/fsnotify"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
	"go.opencensus.io/trace/propagation"
)

const (
	pauseTimeout = 5 * time.Second // docker pause/unpause
)

// Agent exposes an api to create calls from various parameters and then submit
// those calls, it also exposes a 'safe' shutdown mechanism via its Close method.
// Agent has a few roles:
//	* manage the memory pool for a given server
//	* manage the container lifecycle for calls
//	* execute calls against containers
//	* invoke Start and End for each call appropriately
//
// Overview:
// Upon submission of a call, Agent will start the call's timeout timer
// immediately. If the call is hot, Agent will attempt to find an active hot
// container for that route, and if necessary launch another container.  calls
// will be able to read/write directly from/to a socket in the container. If
// it's necessary to launch a container, first an attempt will be made to try
// to reserve the ram required while waiting for any hot 'slot' to become
// available [if applicable]. If there is an error launching the container, an
// error will be returned provided the call has not yet timed out or found
// another hot 'slot' to execute in [if applicable]. call.Start will be called
// immediately before sending any input to a container. call.End will be called
// regardless of the timeout timer's status if the call was executed, and that
// error returned may be returned from Submit.
type Agent interface {
	// GetCall will return a Call that is executable by the Agent, which
	// can be built via various CallOpt's provided to the method.
	GetCall(...CallOpt) (Call, error)

	// Submit will attempt to execute a call locally, a Call may store information
	// about itself in its Start and End methods, which will be called in Submit
	// immediately before and after the Call is executed, respectively. An error
	// will be returned if there is an issue executing the call or the error
	// may be from the call's execution itself (if, say, the container dies,
	// or the call times out).
	Submit(Call) error

	// Close will wait for any outstanding calls to complete and then exit.
	// Closing the agent will invoke Close on the underlying DataAccess.
	// Close is not safe to be called from multiple threads.
	io.Closer

	AddCallListener(fnext.CallListener)
}

type agent struct {
	cfg           Config
	callListeners []fnext.CallListener

	driver drivers.Driver

	slotMgr *slotQueueMgr
	evictor Evictor
	// track usage
	resources ResourceTracker

	// used to track running calls / safe shutdown
	shutWg   *common.WaitGroup
	shutonce sync.Once

	// TODO(reed): shoot this fucking thing
	callOverrider CallOverrider

	// additional options to configure each call
	callOpts []CallOpt

	// deferred actions to call at end of initialisation
	onStartup []func()
}

// Option configures an agent at startup
type Option func(*agent) error

// RegistryToken is a reserved call extensions key to pass registry token
/* #nosec */
const RegistryToken = "FN_REGISTRY_TOKEN"

// New creates an Agent that executes functions locally as Docker containers.
func New(options ...Option) Agent {

	cfg, err := NewConfig()
	if err != nil {
		logrus.WithError(err).Fatalf("error in agent config cfg=%+v", cfg)
	}

	a := &agent{
		cfg: *cfg,
	}

	a.shutWg = common.NewWaitGroup()
	a.slotMgr = NewSlotQueueMgr()
	a.evictor = NewEvictor()

	// Allow overriding config
	for _, option := range options {
		err = option(a)
		if err != nil {
			logrus.WithError(err).Fatal("error in agent options")
		}
	}

	logrus.Infof("agent starting cfg=%+v", a.cfg)

	if a.driver == nil {
		d, err := NewDockerDriver(&a.cfg)
		if err != nil {
			logrus.WithError(err).Fatal("failed to create docker driver ")
		}
		a.driver = d
	}

	a.resources = NewResourceTracker(&a.cfg)

	for _, sup := range a.onStartup {
		sup()
	}
	return a
}

func (a *agent) addStartup(sup func()) {
	a.onStartup = append(a.onStartup, sup)

}

// WithConfig sets the agent config to the provided config
func WithConfig(cfg *Config) Option {
	return func(a *agent) error {
		a.cfg = *cfg
		return nil
	}
}

// WithDockerDriver Provides a customer driver to agent
func WithDockerDriver(drv drivers.Driver) Option {
	return func(a *agent) error {
		if a.driver != nil {
			return errors.New("cannot add driver to agent, driver already exists")
		}

		a.driver = drv
		return nil
	}
}

// WithCallOverrider registers register a CallOverrider to modify a Call and extensions on call construction
func WithCallOverrider(fn CallOverrider) Option {
	return func(a *agent) error {
		if a.callOverrider != nil {
			return errors.New("lb-agent call overriders already exists")
		}
		a.callOverrider = fn
		return nil
	}
}

// WithCallOptions adds additional call options to each call created from GetCall, these
// options will be executed after any other options supplied to GetCall
func WithCallOptions(opts ...CallOpt) Option {
	return func(a *agent) error {
		a.callOpts = append(a.callOpts, opts...)
		return nil
	}
}

// NewDockerDriver creates a default docker driver from agent config
func NewDockerDriver(cfg *Config) (drivers.Driver, error) {
	return drivers.New("docker", drivers.Config{
		DockerNetworks:                cfg.DockerNetworks,
		DockerLoadFile:                cfg.DockerLoadFile,
		ServerVersion:                 cfg.MinDockerVersion,
		PreForkPoolSize:               cfg.PreForkPoolSize,
		PreForkImage:                  cfg.PreForkImage,
		PreForkCmd:                    cfg.PreForkCmd,
		PreForkUseOnce:                cfg.PreForkUseOnce,
		PreForkNetworks:               cfg.PreForkNetworks,
		MaxTmpFsInodes:                cfg.MaxTmpFsInodes,
		EnableReadOnlyRootFs:          !cfg.DisableReadOnlyRootFs,
		ContainerLabelTag:             cfg.ContainerLabelTag,
		ImageCleanMaxSize:             cfg.ImageCleanMaxSize,
		ImageCleanExemptTags:          cfg.ImageCleanExemptTags,
		ImageEnableVolume:             cfg.ImageEnableVolume,
		DisableUnprivilegedContainers: cfg.DisableUnprivilegedContainers,
	})
}

func (a *agent) Close() error {
	var err error

	// wait for ongoing sessions
	a.shutWg.CloseGroup()

	a.shutonce.Do(func() {
		// now close docker layer
		if a.driver != nil {
			err = a.driver.Close()
		}
	})

	return err
}

func (a *agent) Submit(callI Call) error {
	call := callI.(*call)

	ctx := call.req.Context()

	callIDKey, err := tag.NewKey("agent.call_id")
	if err != nil {
		return err
	}
	ctx, err = tag.New(ctx, tag.Insert(callIDKey, call.ID))
	if err != nil {
		return err
	}

	ctx, span := trace.StartSpan(ctx, "agent_submit")
	defer span.End()

	span.AddAttributes(
		trace.StringAttribute("fn.call_id", call.ID),
		trace.StringAttribute("fn.app_id", call.AppID),
		trace.StringAttribute("fn.fn_id", call.FnID),
	)
	rid := common.RequestIDFromContext(ctx)
	if rid != "" {
		span.AddAttributes(
			trace.StringAttribute("fn.rid", rid),
		)
	}

	return a.submit(ctx, call)
}

func (a *agent) startStateTrackers(ctx context.Context, call *call) {
	call.requestState = NewRequestState()
}

func (a *agent) endStateTrackers(ctx context.Context, call *call) {
	call.requestState.UpdateState(ctx, RequestStateDone, call.slots)
}

func (a *agent) submit(ctx context.Context, call *call) error {
	statsCalls(ctx)

	if !a.shutWg.AddSession(1) {
		statsTooBusy(ctx)
		return models.ErrCallTimeoutServerBusy
	}
	defer a.shutWg.DoneSession()

	statsEnqueue(ctx)

	a.startStateTrackers(ctx, call)
	defer a.endStateTrackers(ctx, call)

	slot, err := a.getSlot(ctx, call)
	if err != nil {
		return a.handleCallEnd(ctx, call, slot, err, false)
	}

	err = call.Start(ctx)
	if err != nil {
		return a.handleCallEnd(ctx, call, slot, err, false)
	}

	statsDequeue(ctx)
	statsStartRun(ctx)

	// We are about to execute the function, set container Exec Deadline (call.Timeout)
	slotCtx, cancel := context.WithTimeout(ctx, time.Duration(call.Timeout)*time.Second)
	defer cancel()

	// Pass this error (nil or otherwise) to end directly, to store status, etc.
	err = slot.exec(slotCtx, call)
	return a.handleCallEnd(ctx, call, slot, err, true)
}

func (a *agent) handleCallEnd(ctx context.Context, call *call, slot Slot, err error, isStarted bool) error {

	if slot != nil {
		slot.Close()
	}

	// This means call was routed (executed)
	if isStarted {
		call.End(ctx, err)
		statsStopRun(ctx)
		if err == nil {
			statsComplete(ctx)
		} else if err == context.DeadlineExceeded {
			statsTimedout(ctx)
			return models.ErrCallTimeout
		}
	} else {
		statsDequeue(ctx)
		if err == models.ErrCallTimeoutServerBusy || err == context.DeadlineExceeded {
			statsTooBusy(ctx)
			return models.ErrCallTimeoutServerBusy
		}
	}

	if err == context.Canceled {
		statsCanceled(ctx)
	} else if err != nil {
		statsErrors(ctx)
	}
	return err
}

// getSlot returns a Slot (or error) for the request to run. This will wait
// for other containers to become idle or it may wait for resources to become
// available to launch a new container.
func (a *agent) getSlot(ctx context.Context, call *call) (Slot, error) {
	ctx, span := trace.StartSpan(ctx, "agent_get_slot")
	defer span.End()

	// For hot requests, we use a long lived slot queue, which we use to manage hot containers
	var isNew bool

	if call.slotHashId == "" {
		slotExtns := a.driver.GetSlotKeyExtensions(call.Extensions())
		call.slotHashId = getSlotQueueKey(call, slotExtns)
	}

	call.slots, isNew = a.slotMgr.getSlotQueue(call.slotHashId)
	call.requestState.UpdateState(ctx, RequestStateWait, call.slots)

	// setup slot caller with a ctx that gets cancelled once waitHot() is completed.
	// This allows runHot() to detect if original caller has been serviced by
	// another container or if original caller was disconnected.
	caller := &slotCaller{}
	{
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		caller.id = call.ID
		caller.done = ctx.Done()
		caller.notify = make(chan error, 1)
	}

	// update registry token of the slot queue
	if call.extensions != nil {
		registryToken := call.extensions[RegistryToken]
		if registryToken != "" {
			call.slots.setAuthToken(registryToken)
		}
	}

	if isNew {
		go a.hotLauncher(ctx, call, caller)
	}
	s, err := a.waitHot(ctx, call, caller)
	return s, err
}

// hotLauncher is spawned in a go routine for each slot queue to monitor stats and launch hot
// containers if needed. Upon shutdown or activity timeout, hotLauncher exits and during exit,
// it destroys the slot queue.
func (a *agent) hotLauncher(ctx context.Context, call *call, caller *slotCaller) {
	// Let use 60 minutes or 2 * IdleTimeout as hot queue idle timeout, pick
	// whichever is longer. If in this time, there's no activity, then
	// we destroy the hot queue.
	timeout := a.cfg.HotLauncherTimeout
	idleTimeout := time.Duration(call.IdleTimeout) * time.Second * 2
	if timeout < idleTimeout {
		timeout = idleTimeout
	}

	logger := common.Logger(ctx)
	logger.WithField("launcher_timeout", timeout).Debug("Hot function launcher starting")

	// IMPORTANT: get a context that has a child span / logger but NO timeout
	// TODO this is a 'FollowsFrom'
	var cancel func()
	ctx, cancel = context.WithCancel(common.BackgroundContext(ctx))
	defer cancel()

	ctx, span := trace.StartSpan(ctx, "agent_hot_launcher")
	defer span.End()

	// trigger ctx cancel if server shutdown
	go func() {
		defer cancel()
		select {
		case <-a.shutWg.Closer(): // server shutdown
		case <-ctx.Done():
		}
	}()

	for {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		a.checkLaunch(ctx, call, *caller)

		select {
		case <-ctx.Done(): // timed out
			cancel()
			if a.slotMgr.deleteSlotQueue(call.slots) {
				logger.Debug("Hot function launcher timed out")
				return
			}
		case caller = <-call.slots.signaller:
			cancel()
		}
	}
}

func tryNotify(notifyChan chan error, err error) {
	if notifyChan != nil && err != nil {
		select {
		case notifyChan <- err:
		default:
		}
	}
}

func (a *agent) checkLaunch(ctx context.Context, call *call, caller slotCaller) {
	curStats := call.slots.getStats()
	isBlocking := !a.cfg.EnableNBResourceTracker
	if !isNewContainerNeeded(&curStats) {
		return
	}

	// IMPORTANT: we are here because: isNewContainerNeeded is true,
	// in other words, we need to launch a new container at this time due to high load.

	state := NewContainerState()
	state.UpdateState(ctx, ContainerStateWait, call)

	mem := call.Memory + uint64(call.TmpFsSize)

	var notifyChans []chan struct{}
	var tok ResourceToken

	// For blocking-mode, we wait on a channel for CPU/MEM for cfg.HotPoll duration.
	// If the request is not satisfied during this wait, we perform a non-blocking
	// GetResourceToken()) in an attempt to determine how much mem/cpu we need to evict.
	if isBlocking {
		ctx, cancel := context.WithTimeout(ctx, a.cfg.HotPoll)
		tok = a.resources.GetResourceToken(ctx, mem, call.CPUs)
		cancel()
	}
	if tok == nil {
		tok = a.resources.GetResourceTokenNB(ctx, mem, call.CPUs)
	}

	if tok != nil {
		if tok.Error() != nil {
			if tok.Error() != CapacityFull {
				tryNotify(caller.notify, tok.Error())
			} else {
				needMem, needCpu := tok.NeededCapacity()
				notifyChans = a.evictor.PerformEviction(call.slotHashId, needMem, uint64(needCpu))
				// For Non-blocking mode, if there's nothing to evict, we emit 503.
				if len(notifyChans) == 0 && !isBlocking {
					tryNotify(caller.notify, models.ErrCallTimeoutServerBusy)
				}
			}
		} else if a.shutWg.AddSession(1) {
			go func() {
				// NOTE: runHot will not inherit the timeout from ctx (ignore timings)
				a.runHot(ctx, caller, call, tok, state)
				a.shutWg.DoneSession()
			}()
			// early return (do not allow container state to switch to ContainerStateDone)
			return
		}
		statsUtilization(ctx, a.resources.GetUtilization())
		tok.Close()
	}

	defer state.UpdateState(ctx, ContainerStateDone, call)

	// IMPORTANT: we wait here for any possible evictions to finalize. Otherwise
	// hotLauncher could call checkLaunch again and cause a capacity full (http 503)
	// error.
	for _, wait := range notifyChans {
		select {
		case <-wait:
		case <-ctx.Done(): // timeout
			return
		}
	}
}

// waitHot pings and waits for a hot container from the slot queue
func (a *agent) waitHot(ctx context.Context, call *call, caller *slotCaller) (Slot, error) {
	ctx, span := trace.StartSpan(ctx, "agent_wait_hot")
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // shut down dequeuer if we grab a slot

	ch := call.slots.startDequeuer(ctx)

	// 1) if we can get a slot immediately, grab it.
	// 2) if we don't, send a signaller every x msecs until we do.

	timer := common.NewTimer(1 * time.Microsecond) // pad, so time.After doesn't send immediately
	defer timer.Stop()

	for {
		select {
		case err := <-caller.notify:
			// log everything except for 503
			if err != nil && err != models.ErrCallTimeoutServerBusy {
				common.Logger(ctx).WithError(err).Info("container wait error, sending error to client")
			}
			return nil, err
		case s := <-ch:
			if call.slots.acquireSlot(s) {
				return s.slot, nil
			}
			// we failed to take ownership of the token (eg. container idle timeout) => try again
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-a.shutWg.Closer(): // server shutdown
			return nil, models.ErrCallTimeoutServerBusy
		case <-timer.C:
			// ping dequeuer again
		}

		// set sleep to x msecs after first iteration
		timer.Reset(a.cfg.HotPoll)
		// send a notification to launchHot()
		select {
		case call.slots.signaller <- caller:
		default:
		}
	}
}

// implements Slot
type hotSlot struct {
	done          chan error // signal we are done with slot
	container     *container // TODO mask this
	cfg           *Config
	containerSpan trace.SpanContext
}

func (s *hotSlot) SetError(err error) {
	select {
	case s.done <- err:
	default:
	}
}

func (s *hotSlot) Close() {
	s.SetError(nil)
}

func (s *hotSlot) exec(ctx context.Context, call *call) error {
	ctx, span := trace.StartSpan(ctx, "agent_hot_exec")
	defer span.End()

	call.requestState.UpdateState(ctx, RequestStateExec, call.slots)

	// link the container id and id in the logs [for us!]
	common.Logger(ctx).WithField("container_id", s.container.id).Info("starting call")

	// link the container span to ours for additional context (start/freeze/etc.)
	span.AddLink(trace.Link{
		TraceID: s.containerSpan.TraceID,
		SpanID:  s.containerSpan.SpanID,
		Type:    trace.LinkTypeChild,
	})

	call.req = call.req.WithContext(ctx) // TODO this is funny biz reed is bad
	err := s.container.BeforeCall(ctx, call.Model(), call.Extensions())
	if err != nil {
		// container cannot continue if before call fails
		s.SetError(err)
		return err
	}
	err = s.dispatch(ctx, call)
	err2 := s.container.AfterCall(ctx, call.Model(), call.Extensions())
	if err == nil {
		err = err2
	}
	if err2 != nil {
		// regardless of dispatch errors, container cannot continue if after call fails
		s.SetError(err2)
	}
	return err
}

func createUDSRequest(ctx context.Context, call *call) *http.Request {
	req, err := http.NewRequest("POST", "http://localhost/call", call.req.Body)
	if err != nil {
		common.Logger(ctx).WithError(err).Error("somebody put a bad url in the call http request. 10 lashes.")
		panic(err)
	}
	// Set the context on the request to make sure transport and client handle
	// it properly and close connections at the end, e.g. when using UDS.
	req = req.WithContext(ctx)

	// remove transport headers before passing to function
	common.StripHopHeaders(call.req.Header)

	req.Header = make(http.Header)
	for k, vs := range call.req.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	req.Header.Set("Fn-Call-Id", call.ID)
	deadline, ok := ctx.Deadline()
	if ok {
		deadlineStr := deadline.Format(time.RFC3339)
		req.Header.Set("Fn-Deadline", deadlineStr)
	}

	return req
}

func (s *hotSlot) dispatch(ctx context.Context, call *call) error {
	ctx, span := trace.StartSpan(ctx, "agent_dispatch_httpstream")
	defer span.End()

	// TODO it's possible we can get rid of this (after getting rid of logs API) - may need for call id/debug mode still
	// TODO there's a timeout race for swapping this back if the container doesn't get killed for timing out, and don't you forget it
	swapBack := s.container.swap(call.stderr, &call.Stats)
	defer swapBack()

	req := createUDSRequest(ctx, call)

	var resp *http.Response
	var err error
	{ // don't leak ctx scope
		ctx, span := trace.StartSpan(ctx, "agent_dispatch_uds_do")
		req = req.WithContext(ctx)
		resp, err = s.container.udsClient.Do(req)
		span.End()
	}

	if err != nil {
		// IMPORTANT: Container contract: If http-uds errors/timeout, container cannot continue
		s.SetError(err)
		// first filter out timeouts
		if ctx.Err() == context.DeadlineExceeded {
			return context.DeadlineExceeded
		}
		if strings.Contains(err.Error(), "server response headers exceeded ") {
			return models.ErrFunctionResponseHdrTooBig
		}
		return models.ErrFunctionResponse
	}
	defer resp.Body.Close()

	common.Logger(ctx).WithField("resp", resp).Debug("Got resp from UDS socket")

	ioErrChan := make(chan error, 1)
	go func() {
		ioErrChan <- s.writeResp(ctx, s.cfg.MaxResponseSize, resp, call.respWriter)
	}()

	select {
	case ioErr := <-ioErrChan:
		return ioErr
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			// IMPORTANT: Container contract: If http-uds timeout, container cannot continue
			s.SetError(ctx.Err())
		}
		return ctx.Err()
	}
}

func (s *hotSlot) writeResp(ctx context.Context, max uint64, resp *http.Response, w io.Writer) error {
	rw, ok := w.(http.ResponseWriter)
	if !ok {
		// TODO(reed): this is strange, we should just enforce the response writer type?
		w = common.NewClampWriter(w, max, models.ErrFunctionResponseTooBig)
		return resp.Write(w)
	}

	// IMPORTANT: Container contract: Enforce 200/502/504 expections
	switch resp.StatusCode {
	case http.StatusOK:
		// FDK processed the request OK
	case http.StatusBadGateway:
		// FDK detected failure, container can continue
		return models.ErrFunctionFailed
	case http.StatusGatewayTimeout:
		// FDK detected timeout, respond as if ctx expired, this gets translated & handled in handleCallEnd()
		return context.DeadlineExceeded
	default:
		// Any other code. Possible FDK failure. We shutdown the container
		s.SetError(fmt.Errorf("FDK Error, invalid status code %d", resp.StatusCode))
		return models.ErrFunctionInvalidResponse
	}

	rw = newSizerRespWriter(max, rw)

	// remove transport headers before copying to client response
	common.StripHopHeaders(resp.Header)

	// WARNING: is the following header copy safe?
	// if we're writing directly to the response writer, we need to set headers
	// and only copy the body. resp.Write would copy a full
	// http request into the response body (not what we want).
	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(http.StatusOK)

	_, ioErr := io.Copy(rw, resp.Body)
	return ioErr
}

// XXX(reed): this is a remnant of old io.pipe plumbing, we need to get rid of
// the buffers from the front-end in actuality, but only after removing other formats... so here, eat this
type sizerRespWriter struct {
	http.ResponseWriter
	w io.Writer
}

var _ http.ResponseWriter = new(sizerRespWriter)

func newSizerRespWriter(max uint64, rw http.ResponseWriter) http.ResponseWriter {
	return &sizerRespWriter{
		ResponseWriter: rw,
		w:              common.NewClampWriter(rw, max, models.ErrFunctionResponseTooBig),
	}
}

func (s *sizerRespWriter) Write(b []byte) (int, error) { return s.w.Write(b) }

// Attempt to queue/transmit the error to client
func runHotFailure(ctx context.Context, err error, caller slotCaller) {
	common.Logger(ctx).WithError(err).Info("hot function failure")
	tryNotify(caller.notify, err)
}

func (a *agent) runHot(ctx context.Context, caller slotCaller, call *call, tok ResourceToken, state ContainerState) {
	// IMPORTANT: get a context that has a child span / logger but NO timeout
	// TODO this is a 'FollowsFrom'
	ctx = common.BackgroundContext(ctx)
	ctx, span := trace.StartSpan(ctx, "agent_run_hot")
	defer span.End()

	var childDone chan struct{} // if not nil, a closed channel means child go-routine is done
	var container *container
	var cookie drivers.Cookie
	var err error

	ctrCreatePrepStart := time.Now()

	id := id.New().String()
	logger := logrus.WithFields(logrus.Fields{"container_id": id, "app_id": call.AppID, "fn_id": call.FnID, "image": call.Image, "memory": call.Memory, "cpus": call.CPUs, "idle_timeout": call.IdleTimeout})
	ctx, cancel := context.WithCancel(common.WithLogger(ctx, logger))

	initialized := make(chan struct{}) // when closed, container is ready to handle requests
	udsWait := make(chan error, 1)     // track UDS state and errors

	statsUtilization(ctx, a.resources.GetUtilization())
	state.UpdateState(ctx, ContainerStateStart, call)

	// stack unwind spelled out with strict ordering below.
	defer func() {
		select {
		case <-initialized:
		default:
			runHotFailure(ctx, models.ErrContainerInitFail, caller)
		}

		// shutdown the container and related I/O operations and go routines
		cancel()

		// IMPORTANT: for release cookie (remove container), make sure ctx below has no timeout.
		if cookie != nil {
			cookie.Close(common.BackgroundContext(ctx))
		}

		if container != nil {
			// if there was a child, then let's wait for it, this avoids container.Close()
			// synchronization issues since child may still be modifying the container.
			if childDone != nil {
				<-childDone
			}
			container.Close()
		}

		tok.Close() // release cpu/mem

		state.UpdateState(ctx, ContainerStateDone, call)
		statsUtilization(ctx, a.resources.GetUtilization())
	}()

	// Monitor initialization. Closes 'initialized' channel
	// to hand over the processing to main request processing go-routine
	go func() {
		for {
			select {
			case err := <-udsWait:
				if err != nil {
					runHotFailure(ctx, err, caller)
					cancel()
				} else {
					close(initialized)
				}
				return
			case <-ctx.Done(): // container shutdown
				return
			case <-a.shutWg.Closer(): // agent shutdown
				cancel()
				return
			}
		}
	}()

	// fetch the latest token here. nil check for test pass
	var authToken string
	if call.slots != nil {
		authToken = call.slots.getAuthToken()
	}

	container = newHotContainer(ctx, a.evictor, &caller, call, &a.cfg, id, authToken, udsWait)
	if container == nil {
		return
	}

	cookie, err = a.driver.CreateCookie(ctx, container)
	if err != nil {
		runHotFailure(ctx, err, caller)
		return
	}

	needsPull, err := cookie.ValidateImage(ctx)
	atomic.StoreInt64(&call.ctrPrepTime, int64(time.Since(ctrCreatePrepStart)))
	if needsPull {
		waitStart := time.Now()
		pullCtx, pullCancel := context.WithTimeout(ctx, a.cfg.HotPullTimeout)
		err = cookie.PullImage(pullCtx)
		pullCancel()
		if err != nil {
			if pullCtx.Err() == context.DeadlineExceeded {
				err = models.ErrDockerPullTimeout
			}
		} else {
			needsPull, err = cookie.ValidateImage(ctx) // uses original ctx timeout
			if needsPull {
				// Image must have removed by image cleaner, manual intervention, etc.
				err = models.ErrCallTimeoutServerBusy
			}
		}
		atomic.StoreInt64(&call.imagePullWaitTime, int64(time.Since(waitStart)))
	}
	if err != nil {
		runHotFailure(ctx, err, caller)
		return
	}

	ctrCreateStart := time.Now()
	err = cookie.CreateContainer(ctx)
	if err != nil {
		runHotFailure(ctx, err, caller)
		return
	}

	waiter, err := cookie.Run(ctx)
	if err != nil {
		runHotFailure(ctx, err, caller)
		return
	}
	atomic.StoreInt64(&call.ctrCreateTime, int64(time.Since(ctrCreateStart)))

	childDone = make(chan struct{})

	// Main request processing go-routine
	go func() {
		defer close(childDone)
		defer cancel() // also close if we get an agent shutdown / idle timeout

		// We record init wait for three basic states below: "initialized", "canceled", "timedout"
		// Notice how we do not distinguish between agent-shutdown, eviction, ctx.Done, etc. This is
		// because monitoring go-routine may pick these events earlier and cancel the ctx.
		initStart := time.Now()

		timer := common.NewTimer(a.cfg.HotStartTimeout)
		defer timer.Stop()

		// INIT BARRIER HERE. Wait for the initialization go-routine signal
		select {
		case <-initialized:
			initTime := time.Now() // Declaring this prior to keep the stats in sync
			statsContainerUDSInitLatency(ctx, initStart, initTime, "initialized")
			atomic.StoreInt64(&call.initStartTime, int64(initTime.Sub(initStart)))
		case <-a.shutWg.Closer(): // agent shutdown
			closerTime := time.Now()
			statsContainerUDSInitLatency(ctx, initStart, closerTime, "canceled")
			atomic.StoreInt64(&call.initStartTime, int64(closerTime.Sub(initStart)))
			return
		case <-ctx.Done():
			ctxCancelTime := time.Now()
			statsContainerUDSInitLatency(ctx, initStart, ctxCancelTime, "canceled")
			atomic.StoreInt64(&call.initStartTime, int64(ctxCancelTime.Sub(initStart)))
			return
		case <-timer.C:
			timeoutTime := time.Now()
			statsContainerUDSInitLatency(ctx, initStart, timeoutTime, "timedout")
			atomic.StoreInt64(&call.initStartTime, int64(timeoutTime.Sub(initStart)))
			runHotFailure(ctx, models.ErrContainerInitTimeout, caller)
			return
		}

		timer.Stop() // no longer needed

		for ctx.Err() == nil {
			slot := &hotSlot{
				done:          make(chan error, 1),
				container:     container,
				cfg:           &a.cfg,
				containerSpan: trace.FromContext(ctx).SpanContext(),
			}

			if !a.runHotReq(ctx, call, state, logger, cookie, slot, container) {
				return
			}

			// wait for this call to finish
			// NOTE do NOT select with shutdown / other channels. slot handles this.
			if err := <-slot.done; err != nil {
				logger.WithError(err).Info("hot function terminating")
				return
			}
		}
	}()

	runRes := waiter.Wait(ctx)
	if runRes != nil && runRes.Error() != context.Canceled {
		logger.WithError(runRes.Error()).Info("hot function terminated")
	}
}

//checkSocketDestination verifies that the socket file created by the FDK is valid and permitted - notably verifying that any symlinks are relative to the socket dir
func checkSocketDestination(filename string) error {
	finfo, err := os.Lstat(filename)
	if err != nil {
		return fmt.Errorf("error statting unix socket link file %s", err)
	}

	if (finfo.Mode() & os.ModeSymlink) > 0 {
		linkDest, err := os.Readlink(filename)
		if err != nil {
			return fmt.Errorf("error reading unix socket symlink destination %s", err)
		}
		if filepath.Dir(linkDest) != "." {
			return fmt.Errorf("invalid unix socket symlink, symlinks must be relative within the unix socket directory")
		}
	}

	// stat the absolute path and check it is a socket
	absInfo, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("unable to stat unix socket file %s", err)
	}
	if absInfo.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("listener file is not a socket")
	}

	return nil
}

func inotifyAwait(ctx context.Context, iofsDir string, udsWait chan error) {
	ctx, span := trace.StartSpan(ctx, "inotify_await")
	defer span.End()

	logger := common.Logger(ctx)

	// Here we create the fs notify (inotify) synchronously and once that is
	// setup, then fork off our async go-routine. Basically fsnotify should be enabled
	// before we launch the container in order not to miss any events.
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		udsWait <- fmt.Errorf("error getting fsnotify watcher: %v", err)
		return
	}

	err = fsWatcher.Add(iofsDir)
	if err != nil {
		if err := fsWatcher.Close(); err != nil {
			logger.WithError(err).Error("Failed to close inotify watcher")
		}
		udsWait <- fmt.Errorf("error adding iofs dir to fswatcher: %v", err)
		return
	}

	go func() {
		ctx, span := trace.StartSpan(ctx, "inotify_await_poller")
		defer span.End()

		defer func() {
			if err := fsWatcher.Close(); err != nil {
				logger.WithError(err).Error("Failed to close inotify watcher")
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case err := <-fsWatcher.Errors:
				// TODO: We do not know if these cases would be due to customer container/FDK
				// fault or some kind of service/runner issue. As conservative choice,
				// we reflect back a non API error, which means a 500 back to user.
				logger.WithError(err).Error("error watching for iofs")
				udsWait <- err
				return
			case event := <-fsWatcher.Events:
				logger.WithField("event", event).Debug("fsnotify event")
				if event.Op&fsnotify.Create == fsnotify.Create && event.Name == filepath.Join(iofsDir, udsFilename) {
					err := checkSocketDestination(filepath.Join(iofsDir, udsFilename))
					if err != nil {
						// This case is more like a bad FDK/container, so let's reflect this back to
						// clients as container init fail.
						logger.WithError(err).Error("Failed to check socket destination")
						udsWait <- models.ErrContainerInitFail
					} else {
						close(udsWait)
					}
					return
				}
			}
		}
	}()
}

// runHotReq enqueues a free slot to slot queue manager and watches various timers and the consumer until
// the slot is consumed. A return value of false means, the container should shutdown and no subsequent
// calls should be made to this function.
func (a *agent) runHotReq(ctx context.Context, call *call, state ContainerState, logger logrus.FieldLogger, cookie drivers.Cookie, slot *hotSlot, c *container) bool {

	var err error
	isFrozen := false

	freezeTimer := common.NewTimer(a.cfg.FreezeIdle)
	idleTimer := common.NewTimer(time.Duration(call.IdleTimeout) * time.Second)

	defer func() {
		freezeTimer.Stop()
		idleTimer.Stop()
		// log if any error is encountered
		if err != nil {
			logger.WithError(err).Error("hot function failure")
		}
	}()

	state.UpdateState(ctx, ContainerStateIdle, call)
	c.EnableEviction(call)

	s := call.slots.queueSlot(slot)

	// WARNING: Do not hold on to channel after calling Enable/DisableEviction
	evicted := c.GetEvictChan()

	for {
		select {
		case <-s.trigger: // slot already consumed
		case <-ctx.Done(): // container shutdown
		case <-a.shutWg.Closer(): // agent shutdown
		case <-idleTimer.C:
		case <-freezeTimer.C:
			if !isFrozen {
				ctx, cancel := context.WithTimeout(ctx, pauseTimeout)
				err = cookie.Freeze(ctx)
				cancel()
				if err != nil {
					return false
				}
				isFrozen = true
				state.UpdateState(ctx, ContainerStatePaused, call)
			}
			continue
		case <-evicted:
		}
		break
	}

	// if we can acquire token, that means we are here due to
	// abort/shutdown/timeout/evict, attempt to acquire and terminate,
	// otherwise continue processing the request
	if call.slots.acquireSlot(s) {
		select {
		case <-evicted:
			statsContainerEvicted(ctx, state.GetState())
		default:
		}
		return false
	}

	// We disable eviction after acquisition attempt above, since
	// this can reinstall an eviction token if eviction has taken place.
	c.DisableEviction(call)

	// In case, timer/acquireSlot failure landed us here, make
	// sure to unfreeze.
	if isFrozen {
		ctx, cancel := context.WithTimeout(ctx, pauseTimeout)
		err = cookie.Unfreeze(ctx)
		cancel()
		if err != nil {
			return false
		}
		isFrozen = false
	}

	state.UpdateState(ctx, ContainerStateBusy, call)
	return true
}

// container implements drivers.ContainerTask container is the execution of a
// single container, which may run multiple functions [consecutively]. the id
// and stderr can be swapped out by new calls in the container.  input and
// output must be copied in and out. stdout is sent to stderr.
type container struct {
	id             string // contrived
	image          string
	env            map[string]string
	extensions     map[string]string
	memory         uint64
	cpus           uint64
	fsSize         uint64
	pids           uint64
	openFiles      *uint64
	lockedMemory   *uint64
	pendingSignals *uint64
	messageQueue   *uint64
	tmpFsSize      uint64
	disableNet     bool
	iofs           iofs
	logCfg         drivers.LoggerConfig
	close          func()
	beforeCall     drivers.BeforeCall
	afterCall      drivers.AfterCall
	dockerAuth     dockerdriver.Auther
	authToken      string

	stderr io.Writer

	udsClient http.Client

	// swapMu protects the stats swapping
	swapMu sync.Mutex
	stats  *driver_stats.Stats

	evictor    Evictor
	evictToken *EvictToken
}

var _ drivers.ContainerTask = &container{}

// newHotContainer creates a container that can be used for multiple sequential events
func newHotContainer(ctx context.Context, evictor Evictor, caller *slotCaller, call *call, cfg *Config, id, authToken string, udsWait chan error) *container {

	var iofs iofs
	var err error

	logger := common.Logger(ctx)

	if cfg.IOFSEnableTmpfs {
		iofs, err = newTmpfsIOFS(ctx, cfg)
	} else {
		iofs, err = newDirectoryIOFS(ctx, cfg)
	}
	if err != nil {
		udsWait <- err
		return nil
	}

	inotifyAwait(ctx, iofs.AgentPath(), udsWait)

	// IMPORTANT: we are not operating on a TTY allocated container. This means, stderr and stdout are multiplexed
	// from the same stream internally via docker using a multiplexing protocol. Therefore, stderr/stdout *BOTH*
	// have to be read or *BOTH* blocked consistently. In other words, we cannot block one and continue
	// reading from the other one without risking head-of-line blocking.

	// TODO(reed): we should let the syslog driver pick this up really but our
	// default story sucks there

	// disable container logs if they're disabled on the call (pure_runner) -
	// users may use syslog to get container logs, unrelated to this writer.
	// otherwise, make a line writer and allow logrus DEBUG logs to host stderr
	// between function invocations from the container.

	var bufs []*bytes.Buffer
	var stderr io.WriteCloser = call.stderr
	if _, ok := stderr.(common.NoopReadWriteCloser); !ok {
		gw := common.NewGhostWriter()
		buf1 := bufPool.Get().(*bytes.Buffer)
		sec := &nopCloser{&logWriter{
			logrus.WithFields(logrus.Fields{"tag": "stderr", "app_id": call.AppID, "fn_id": call.FnID, "image": call.Image, "container_id": id}),
		}}
		gw.Swap(newLineWriterWithBuffer(buf1, sec))
		stderr = gw
		bufs = append(bufs, buf1)
	}

	baseTransport := &http.Transport{
		MaxIdleConns:           1,
		MaxIdleConnsPerHost:    1,
		MaxResponseHeaderBytes: int64(cfg.MaxHdrResponseSize),
		IdleConnTimeout:        1 * time.Second, // TODO(jang): revert this to 120s at the point all FDKs are known to be fixed
		// TODO(reed): since we only allow one, and we close them, this is gratuitous?
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", filepath.Join(iofs.AgentPath(), udsFilename))
		},
	}

	env := cloneStrMap(call.Config) // clone to avoid data race

	// Debug info exposed to FDK/Container
	if cfg.EnableFDKDebugInfo {
		if caller != nil {
			env["FN_SPAWN_CALL_ID"] = caller.id
		}
	}

	return &container{
		id:             id, // XXX we could just let docker generate ids...
		image:          call.Image,
		env:            env,
		extensions:     cloneStrMap(call.extensions), // avoid date race
		memory:         call.Memory,
		cpus:           uint64(call.CPUs),
		fsSize:         cfg.MaxFsSize,
		pids:           uint64(cfg.MaxPIDs),
		openFiles:      cfg.MaxOpenFiles,
		lockedMemory:   cfg.MaxLockedMemory,
		pendingSignals: cfg.MaxPendingSignals,
		messageQueue:   cfg.MaxMessageQueue,
		tmpFsSize:      uint64(call.TmpFsSize),
		disableNet:     call.disableNet,
		iofs:           iofs,
		dockerAuth:     call.dockerAuth,
		authToken:      authToken,
		logCfg: drivers.LoggerConfig{
			URL: strings.TrimSpace(call.SyslogURL),
			Tags: []drivers.LoggerTag{
				{Name: "app_id", Value: call.AppID},
				{Name: "fn_id", Value: call.FnID},
			},
		},
		stderr: stderr,
		udsClient: http.Client{
			// use this transport so we can trace the requests to container, handy for debugging...
			Transport: &ochttp.Transport{
				NewClientTrace: ochttp.NewSpanAnnotatingClientTrace,
				Propagation:    noopOCHTTPFormat{}, // we do NOT want to send our tracers to user function, they default to b3
				Base:           baseTransport,
				// NOTE: the global trace sampler will be used, this is what we want for now at least
			},
		},
		evictor:    evictor,
		beforeCall: func(context.Context, *models.Call, drivers.CallExtensions) error { return nil },
		afterCall:  func(context.Context, *models.Call, drivers.CallExtensions) error { return nil },
		close: func() {
			stderr.Close()
			for _, b := range bufs {
				bufPool.Put(b)
			}
			if err := iofs.Close(); err != nil {
				logger.WithError(err).Error("Error closing IOFS")
			}
			baseTransport.CloseIdleConnections()
		},
	}
}

var _ propagation.HTTPFormat = noopOCHTTPFormat{}

// we do not want to pass these to the user functions, since they're our internal traces...
// it is useful for debugging, admittedly, we could make it more friendly for OSS debugging...
type noopOCHTTPFormat struct{}

func (noopOCHTTPFormat) SpanContextFromRequest(req *http.Request) (sc trace.SpanContext, ok bool) {
	// our transport isn't receiving requests anyway
	return trace.SpanContext{}, false
}
func (noopOCHTTPFormat) SpanContextToRequest(sc trace.SpanContext, req *http.Request) {}

func (c *container) swap(stderr io.Writer, cs *driver_stats.Stats) func() {
	// if they aren't using a ghost writer, the logs are disabled, we can skip swapping
	gw, ok := c.stderr.(common.GhostWriter)
	var ostderr io.Writer
	if ok {
		ostderr = gw.Swap(stderr)
	}
	c.swapMu.Lock()
	ocs := c.stats
	c.stats = cs
	c.swapMu.Unlock()

	return func() {
		if ostderr != nil {
			c.stderr.(common.GhostWriter).Swap(ostderr)
		}
		c.swapMu.Lock()
		c.stats = ocs
		c.swapMu.Unlock()
	}
}

func (c *container) Id() string                         { return c.id }
func (c *container) Command() string                    { return "" }
func (c *container) Input() io.Reader                   { return common.NoopReadWriteCloser{} }
func (c *container) Logger() (io.Writer, io.Writer)     { return c.stderr, c.stderr }
func (c *container) Volumes() [][2]string               { return nil }
func (c *container) WorkDir() string                    { return "" }
func (c *container) Image() string                      { return c.image }
func (c *container) EnvVars() map[string]string         { return c.env }
func (c *container) Memory() uint64                     { return c.memory * 1024 * 1024 } // convert MB
func (c *container) CPUs() uint64                       { return c.cpus }
func (c *container) FsSize() uint64                     { return c.fsSize }
func (c *container) PIDs() uint64                       { return c.pids }
func (c *container) OpenFiles() *uint64                 { return c.openFiles }
func (c *container) LockedMemory() *uint64              { return c.lockedMemory }
func (c *container) PendingSignals() *uint64            { return c.pendingSignals }
func (c *container) MessageQueue() *uint64              { return c.messageQueue }
func (c *container) TmpFsSize() uint64                  { return c.tmpFsSize }
func (c *container) Extensions() map[string]string      { return c.extensions }
func (c *container) LoggerConfig() drivers.LoggerConfig { return c.logCfg }
func (c *container) UDSAgentPath() string               { return c.iofs.AgentPath() }
func (c *container) UDSDockerPath() string              { return c.iofs.DockerPath() }
func (c *container) UDSDockerDest() string              { return iofsDockerMountDest }
func (c *container) DisableNet() bool                   { return c.disableNet }

// WriteStat publishes each metric in the specified Stats structure as a histogram metric
func (c *container) WriteStat(ctx context.Context, stat driver_stats.Stat) {
	for key, value := range stat.Metrics {
		if m, ok := dockerMeasures[key]; ok {
			stats.Record(ctx, m.M(int64(value)))
		}
	}

	c.swapMu.Lock()
	if c.stats != nil {
		*(c.stats) = append(*(c.stats), stat)
	}
	c.swapMu.Unlock()
}

// EnableEviction allows container eviction
func (c *container) EnableEviction(call *call) {
	if c.evictToken == nil {
		c.evictToken = c.evictor.CreateEvictToken(call.slotHashId, call.Memory+uint64(call.TmpFsSize), uint64(call.CPUs))
	}
	c.evictToken.SetEvictable(true)
}

// DisableEviction disables container eviction.
func (c *container) DisableEviction(call *call) {
	if c.evictToken == nil {
		return
	}

	c.evictToken.SetEvictable(false)

	// if we are too late, then delete already evicted token. Let's refresh the token
	select {
	case <-c.evictToken.C:
	default:
		return
	}

	c.evictor.DeleteEvictToken(c.evictToken)
	c.evictToken = c.evictor.CreateEvictToken(call.slotHashId, call.Memory+uint64(call.TmpFsSize), uint64(call.CPUs))
}

// Close closes container and releases resources associated with it
func (c *container) Close() {
	if c.close != nil {
		c.close()
		c.close = nil
	}
	// evictor can be nil (for tests)
	if c.evictor == nil {
		return
	}

	if c.evictToken != nil {
		c.evictor.DeleteEvictToken(c.evictToken)
		c.evictToken = nil
	}
}

// WrapClose adds additional behaviour to the ContainerTask Close() call
func (c *container) WrapClose(wrapper func(closer func()) func()) {
	c.close = wrapper(c.close)
}

func (c *container) BeforeCall(ctx context.Context, call *models.Call, extn drivers.CallExtensions) error {
	return c.beforeCall(ctx, call, extn)
}

// WrapBeforeCall adds additional behaviour to any BeforeCall invocation
func (c *container) WrapBeforeCall(wrapper func(before drivers.BeforeCall) drivers.BeforeCall) {
	c.beforeCall = wrapper(c.beforeCall)
}

func (c *container) AfterCall(ctx context.Context, call *models.Call, extn drivers.CallExtensions) error {
	return c.afterCall(ctx, call, extn)
}

// WrapClose adds additional behaviour to the ContainerTask Close() call
func (c *container) WrapAfterCall(wrapper func(after drivers.AfterCall) drivers.AfterCall) {
	c.afterCall = wrapper(c.afterCall)
}

// GetEvictChan returns a channel that closes if an eviction occurs. Do not
// refer to the same channel after calls to EnableEviction/DisableEviction flags or Close().
func (c *container) GetEvictChan() chan struct{} {
	if c.evictToken != nil {
		return c.evictToken.C
	}
	return nil
}

// assert we implement this at compile time
var _ dockerdriver.Auther = new(container)

// DockerAuth implements the docker.AuthConfiguration interface.
func (c *container) DockerAuth(ctx context.Context, image string) (*docker.AuthConfiguration, error) {
	if c.dockerAuth != nil {
		return c.dockerAuth.DockerAuth(ctx, image)
	}

	registryToken := c.authToken
	if registryToken != "" {
		return &docker.AuthConfiguration{
			RegistryToken: registryToken,
		}, nil
	}
	return nil, nil
}

func cloneStrMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
