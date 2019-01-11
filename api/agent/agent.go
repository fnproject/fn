package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"path/filepath"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/fsnotify/fsnotify"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/trace"
	"os"
)

const (
	pauseTimeout = 5 * time.Second // docker pause/unpause
)

// TODO we should prob store async calls in db immediately since we're returning id (will 404 until post-execution)
// TODO async calls need to add route.Headers as well
// TODO handle timeouts / no response in sync & async (sync is json+503 atm, not 504, async is empty log+status)
// see also: server/runner.go wrapping the response writer there, but need to handle async too (push down?)
// TODO storing logs / call can push call over the timeout
// TODO async is still broken, but way less so. we need to modify mq semantics
// to be much more robust. now we're at least running it if we delete the msg,
// but we may never store info about that execution so still broked (if fn
// dies). need coordination w/ db.
// TODO if async would store requests (or interchange format) it would be slick, but
// if we're going to store full calls in db maybe we should only queue pointers to ids?
// TODO examine cases where hot can't start a container and the user would never see an error
// about why that may be so (say, whatever it is takes longer than the timeout, e.g.)
// TODO if an image is not found or similar issues in getting a slot, then async should probably
// mark the call as errored rather than forever trying & failing to run it
// TODO it would be really nice if we made the ramToken wrap the driver cookie (less brittle,
// if those leak the container leaks too...) -- not the allocation, but the token.Close and cookie.Close
// TODO if machine is out of ram, just timeout immediately / wait for hot slot? (discuss policy)

// Agent exposes an api to create calls from various parameters and then submit
// those calls, it also exposes a 'safe' shutdown mechanism via its Close method.
// Agent has a few roles:
//	* manage the memory pool for a given server
//	* manage the container lifecycle for calls
//	* execute calls against containers
//	* invoke Start and End for each call appropriately
//	* check the mq for any async calls, and submit them
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
	da            CallHandler
	callListeners []fnext.CallListener

	driver drivers.Driver

	slotMgr *slotQueueMgr
	evictor Evictor
	// track usage
	resources ResourceTracker

	// used to track running calls / safe shutdown
	shutWg   *common.WaitGroup
	shutonce sync.Once

	callOverrider CallOverrider
	// deferred actions to call at end of initialisation
	onStartup []func()
}

// Option configures an agent at startup
type Option func(*agent) error

// RegistryToken is a reserved call extensions key to pass registry token
/* #nosec */
const RegistryToken = "FN_REGISTRY_TOKEN"

// New creates an Agent that executes functions locally as Docker containers.
func New(da CallHandler, options ...Option) Agent {

	cfg, err := NewConfig()
	if err != nil {
		logrus.WithError(err).Fatalf("error in agent config cfg=%+v", cfg)
	}

	a := &agent{
		cfg: *cfg,
	}

	a.shutWg = common.NewWaitGroup()
	a.da = da
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

// WithAsync Enables Async  operations on the agent
func WithAsync(dqda DequeueDataAccess) Option {
	return func(a *agent) error {
		a.addStartup(func() {
			if !a.shutWg.AddSession(1) {
				logrus.Fatal("cannot start agent, unable to add session")
			}
			go a.asyncDequeue(dqda) // safe shutdown can nanny this fine
		})
		return nil
	}
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

// NewDockerDriver creates a default docker driver from agent config
func NewDockerDriver(cfg *Config) (drivers.Driver, error) {
	return drivers.New("docker", drivers.Config{
		DockerNetworks:       cfg.DockerNetworks,
		DockerLoadFile:       cfg.DockerLoadFile,
		ServerVersion:        cfg.MinDockerVersion,
		PreForkPoolSize:      cfg.PreForkPoolSize,
		PreForkImage:         cfg.PreForkImage,
		PreForkCmd:           cfg.PreForkCmd,
		PreForkUseOnce:       cfg.PreForkUseOnce,
		PreForkNetworks:      cfg.PreForkNetworks,
		MaxTmpFsInodes:       cfg.MaxTmpFsInodes,
		EnableReadOnlyRootFs: !cfg.DisableReadOnlyRootFs,
		MaxRetries:           cfg.MaxDockerRetries,
		ContainerLabelTag:    cfg.ContainerLabelTag,
		ImageCleanMaxSize:    cfg.ImageCleanMaxSize,
		ImageCleanExemptTags: cfg.ImageCleanExemptTags,
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
	ctx, span := trace.StartSpan(call.req.Context(), "agent_submit")
	defer span.End()

	statsCalls(ctx)

	if !a.shutWg.AddSession(1) {
		statsTooBusy(ctx)
		return models.ErrCallTimeoutServerBusy
	}
	defer a.shutWg.DoneSession()

	err := a.submit(ctx, call)
	return err
}

func (a *agent) startStateTrackers(ctx context.Context, call *call) {
	call.requestState = NewRequestState()
}

func (a *agent) endStateTrackers(ctx context.Context, call *call) {
	call.requestState.UpdateState(ctx, RequestStateDone, call.slots)
}

func (a *agent) submit(ctx context.Context, call *call) error {
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
	if call.Type == models.TypeAsync {
		// *) for async, slot deadline is also call.Timeout. This is because we would like to
		// allocate enough time for docker-pull, slot-wait, docker-start, etc.
		// and also make sure we have call.Timeout inside the container. Total time
		// to run an async becomes 2 * call.Timeout.
		// *) for sync, there's no slot deadline, the timeout is controlled by http-client
		// context (or runner gRPC context)
		tmp, cancel := context.WithTimeout(ctx, time.Duration(call.Timeout)*time.Second)
		ctx = tmp
		defer cancel()
	}

	ctx, span := trace.StartSpan(ctx, "agent_get_slot")
	defer span.End()

	// For hot requests, we use a long lived slot queue, which we use to manage hot containers
	var isNew bool

	if call.slotHashId == "" {
		call.slotHashId = getSlotQueueKey(call)
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

		caller.done = ctx.Done()
		caller.notify = make(chan error)
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
	ctx = common.BackgroundContext(ctx)
	ctx, span := trace.StartSpan(ctx, "agent_hot_launcher")
	defer span.End()

	for {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		a.checkLaunch(ctx, call, *caller)

		select {
		case <-a.shutWg.Closer(): // server shutdown
			cancel()
			return
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
	isNB := a.cfg.EnableNBResourceTracker
	if !isNewContainerNeeded(&curStats) {
		return
	}

	state := NewContainerState()
	state.UpdateState(ctx, ContainerStateWait, call.slots)

	mem := call.Memory + uint64(call.TmpFsSize)

	var notifyChans []chan struct{}
	var tok ResourceToken

	// WARNING: Tricky flow below. We are here because: isNewContainerNeeded is true,
	// in other words, we need to launch a new container at this time due to high load.
	//
	// For non-blocking mode, this means, if we cannot acquire resources (cpu+mem), then we need
	// to notify the caller through notifyChan. This is not perfect as the callers and
	// checkLaunch do not match 1-1. But this is OK, we can notify *any* waiter that
	// has signalled us, this is because non-blocking mode is a system wide setting.
	// The notifications are lossy, but callers will signal/poll again if this is the case
	// or this may not matter if they've already acquired an empty slot.
	//
	// For Non-blocking mode, a.cfg.HotPoll should not be set to too high since a missed
	// notify event from here will add a.cfg.HotPoll msec latency. Setting a.cfg.HotPoll may
	// be an acceptable workaround for the short term since non-blocking mode likely to reduce
	// the number of waiters which perhaps could compensate for more frequent polling.
	//
	// Non-blocking mode only applies to cpu+mem, and if isNewContainerNeeded decided that we do not
	// need to start a new container, then waiters will wait.
	select {
	case tok = <-a.resources.GetResourceToken(ctx, mem, call.CPUs, isNB):
	case <-time.After(a.cfg.HotPoll):
		// Request routines are polling us with this a.cfg.HotPoll frequency. We can use this
		// same timer to assume that we waited for cpu/mem long enough. Let's try to evict an
		// idle container. We do this by submitting a non-blocking request and evicting required
		// amount of resources.
		select {
		case tok = <-a.resources.GetResourceToken(ctx, mem, call.CPUs, true):
		case <-ctx.Done(): // timeout
		case <-a.shutWg.Closer(): // server shutdown
		}
	case <-ctx.Done(): // timeout
	case <-a.shutWg.Closer(): // server shutdown
	}

	if tok != nil {
		if tok.Error() != nil {
			if tok.Error() != CapacityFull {
				tryNotify(caller.notify, tok.Error())
			} else {
				needMem, needCpu := tok.NeededCapacity()
				notifyChans = a.evictor.PerformEviction(call.slotHashId, needMem, uint64(needCpu))
				// For Non-blocking mode, if there's nothing to evict, we emit 503.
				if len(notifyChans) == 0 && isNB {
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

	defer state.UpdateState(ctx, ContainerStateDone, call.slots)

	// IMPORTANT: we wait here for any possible evictions to finalize. Otherwise
	// hotLauncher could call checkLaunch again and cause a capacity full (http 503)
	// error.
	for _, wait := range notifyChans {
		select {
		case <-wait:
		case <-ctx.Done(): // timeout
			return
		case <-a.shutWg.Closer(): // server shutdown
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

	sleep := 1 * time.Microsecond // pad, so time.After doesn't send immediately
	for {
		select {
		case err := <-caller.notify:
			return nil, err
		case s := <-ch:
			if call.slots.acquireSlot(s) {
				if s.slot.Error() != nil {
					s.slot.Close()
					return nil, s.slot.Error()
				}
				return s.slot, nil
			}
			// we failed to take ownership of the token (eg. container idle timeout) => try again
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-a.shutWg.Closer(): // server shutdown
			return nil, models.ErrCallTimeoutServerBusy
		case <-time.After(sleep):
			// ping dequeuer again
		}

		// set sleep to x msecs after first iteration
		sleep = a.cfg.HotPoll
		// send a notification to launchHot()
		select {
		case call.slots.signaller <- caller:
		default:
		}
	}
}

// implements Slot
type hotSlot struct {
	done          chan struct{} // signal we are done with slot
	container     *container    // TODO mask this
	cfg           *Config
	fatalErr      error
	containerSpan trace.SpanContext
}

func (s *hotSlot) Close() error {
	close(s.done)
	return nil
}

func (s *hotSlot) Error() error {
	return s.fatalErr
}

func (s *hotSlot) trySetError(err error) {
	if s.fatalErr == nil {
		s.fatalErr = err
	}
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
	return s.dispatch(ctx, call)
}

var removeHeaders = map[string]bool{
	"connection":        true,
	"keep-alive":        true,
	"trailer":           true,
	"transfer-encoding": true,
	"te":                true,
	"upgrade":           true,
	"authorization":     true,
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

	req.Header = make(http.Header)
	for k, vs := range call.req.Header {
		if !removeHeaders[strings.ToLower(k)] {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
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

	resp, err := s.container.udsClient.Do(createUDSRequest(ctx, call))
	if err != nil {
		// IMPORTANT: Container contract: If http-uds errors/timeout, container cannot continue
		s.trySetError(err)
		// first filter out timeouts
		if ctx.Err() == context.DeadlineExceeded {
			return context.DeadlineExceeded
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
			s.trySetError(ctx.Err())
		}
		return ctx.Err()
	}
}

func (s *hotSlot) writeResp(ctx context.Context, max uint64, resp *http.Response, w io.Writer) error {
	rw, ok := w.(http.ResponseWriter)
	if !ok {
		// WARNING: this bypasses container contract translation. Assuming this is
		// async mode, where we are storing response in call.stderr.
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
		s.trySetError(fmt.Errorf("FDK Error, invalid status code %d", resp.StatusCode))
		return models.ErrFunctionInvalidResponse
	}

	rw = newSizerRespWriter(max, rw)
	rw.WriteHeader(http.StatusOK)

	// WARNING: is the following header copy safe?
	// if we're writing directly to the response writer, we need to set headers
	// and only copy the body. resp.Write would copy a full
	// http request into the response body (not what we want).
	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}

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

// Try to queue an error to the error channel if possible.
func tryQueueErr(err error, ch chan error) error {
	if err != nil {
		select {
		case ch <- err:
		default:
		}
	}
	return err
}

func (a *agent) runHot(ctx context.Context, caller slotCaller, call *call, tok ResourceToken, state ContainerState) {
	// IMPORTANT: get a context that has a child span / logger but NO timeout
	// TODO this is a 'FollowsFrom'
	ctx = common.BackgroundContext(ctx)
	ctx, span := trace.StartSpan(ctx, "agent_run_hot")
	defer span.End()

	var container *container
	var cookie drivers.Cookie
	var err error

	id := id.New().String()
	logger := logrus.WithFields(logrus.Fields{"id": id, "app_id": call.AppID, "fn_id": call.FnID, "image": call.Image, "memory": call.Memory, "cpus": call.CPUs, "idle_timeout": call.IdleTimeout})
	ctx, cancel := context.WithCancel(common.WithLogger(ctx, logger))

	initialized := make(chan struct{}) // when closed, container is ready to handle requests
	udsWait := make(chan error, 1)     // track UDS state and errors
	errQueue := make(chan error, 1)    // errors to be reflected back to the slot queue

	evictor := a.evictor.CreateEvictToken(call.slotHashId, call.Memory+uint64(call.TmpFsSize), uint64(call.CPUs))

	statsUtilization(ctx, a.resources.GetUtilization())
	state.UpdateState(ctx, ContainerStateStart, call.slots)

	// stack unwind spelled out with strict ordering below.
	defer func() {
		// IMPORTANT: we ignore any errors due to eviction and do not reflect these to clients.
		if !evictor.isEvicted() {
			select {
			case <-initialized:
			default:
				tryQueueErr(models.ErrContainerInitFail, errQueue)
			}
			select {
			case err := <-errQueue:
				call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
			default:
			}
		}

		// shutdown the container and related I/O operations and go routines
		cancel()

		// IMPORTANT: for release cookie (remove container), make sure ctx below has no timeout.
		if cookie != nil {
			cookie.Close(common.BackgroundContext(ctx))
		}

		if container != nil {
			container.Close()
		}

		lastState := state.GetState()
		state.UpdateState(ctx, ContainerStateDone, call.slots)

		tok.Close() // release cpu/mem

		// IMPORTANT: evict token is deleted *after* resource token.
		// This ordering allows resource token to be freed first, which means once evict token
		// is deleted, eviction is considered to be completed.
		a.evictor.DeleteEvictToken(evictor)

		statsUtilization(ctx, a.resources.GetUtilization())
		if evictor.isEvicted() {
			logger.Debugf("Hot function evicted")
			statsContainerEvicted(ctx, lastState)
		}
	}()

	// Monitor initialization and evictability. Closes 'initialized' channel
	// to hand over the processing to main request processing go-routine
	go func() {
		for {
			select {
			case err := <-udsWait:
				if tryQueueErr(err, errQueue) != nil {
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
			case <-caller.done: // original caller disconnected or serviced by another container?
				evictor.SetEvictable(true)
				caller.done = nil // block 'caller.done' after this point
			case <-evictor.C: // eviction
				cancel()
				return
			}
		}
	}()

	container = newHotContainer(ctx, call, &a.cfg, id, udsWait)
	if container == nil {
		return
	}

	cookie, err = a.driver.CreateCookie(ctx, container)
	if tryQueueErr(err, errQueue) != nil {
		return
	}

	needsPull, err := cookie.ValidateImage(ctx)
	if tryQueueErr(err, errQueue) != nil {
		return
	}

	if needsPull {
		ctx, cancel := context.WithTimeout(ctx, a.cfg.HotPullTimeout)
		err = cookie.PullImage(ctx)
		cancel()
		if ctx.Err() == context.DeadlineExceeded {
			err = models.ErrDockerPullTimeout
		}
		if tryQueueErr(err, errQueue) != nil {
			return
		}
	}

	err = cookie.CreateContainer(ctx)
	if tryQueueErr(err, errQueue) != nil {
		return
	}

	waiter, err := cookie.Run(ctx)
	if tryQueueErr(err, errQueue) != nil {
		return
	}

	// Main request processing go-routine
	go func() {
		defer cancel() // also close if we get an agent shutdown / idle timeout

		// We record init wait for three basic states below: "initialized", "canceled", "timedout"
		// Notice how we do not distinguish between agent-shutdown, eviction, ctx.Done, etc. This is
		// because monitoring go-routine may pick these events earlier and cancel the ctx.
		initStart := time.Now()

		// INIT BARRIER HERE. Wait for the initialization go-routine signal
		select {
		case <-initialized:
			statsContainerUDSInitLatency(ctx, initStart, time.Now(), "initialized")
		case <-a.shutWg.Closer(): // agent shutdown
			statsContainerUDSInitLatency(ctx, initStart, time.Now(), "canceled")
			return
		case <-ctx.Done():
			statsContainerUDSInitLatency(ctx, initStart, time.Now(), "canceled")
			return
		case <-evictor.C: // eviction
			statsContainerUDSInitLatency(ctx, initStart, time.Now(), "canceled")
			return
		case <-time.After(a.cfg.HotStartTimeout):
			statsContainerUDSInitLatency(ctx, initStart, time.Now(), "timedout")
			tryQueueErr(models.ErrContainerInitTimeout, errQueue)
			return
		}

		for {
			// Below we are rather defensive and poll on evictor/ctx
			// to reduce the likelyhood of attempting to queue a hotSlot when these
			// two cases occur.
			select {
			case <-ctx.Done():
				return
			case <-evictor.C: // eviction
				return
			default:
			}

			slot := &hotSlot{
				done:          make(chan struct{}),
				container:     container,
				cfg:           &a.cfg,
				containerSpan: trace.FromContext(ctx).SpanContext(),
			}
			if !a.runHotReq(ctx, call, state, logger, cookie, slot, evictor) {
				return
			}
			// wait for this call to finish
			// NOTE do NOT select with shutdown / other channels. slot handles this.
			<-slot.done

			if slot.fatalErr != nil {
				logger.WithError(slot.fatalErr).Info("hot function terminating")
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
func (a *agent) runHotReq(ctx context.Context, call *call, state ContainerState, logger logrus.FieldLogger, cookie drivers.Cookie, slot *hotSlot, evictor *EvictToken) bool {

	var err error
	isFrozen := false

	freezeTimer := time.NewTimer(a.cfg.FreezeIdle)
	idleTimer := time.NewTimer(time.Duration(call.IdleTimeout) * time.Second)

	defer func() {
		evictor.SetEvictable(false)
		freezeTimer.Stop()
		idleTimer.Stop()
		// log if any error is encountered
		if err != nil {
			logger.WithError(err).Error("hot function failure")
		}
	}()

	evictor.SetEvictable(true)
	state.UpdateState(ctx, ContainerStateIdle, call.slots)

	s := call.slots.queueSlot(slot)

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
				state.UpdateState(ctx, ContainerStatePaused, call.slots)
			}
			continue
		case <-evictor.C:
		}
		break
	}

	evictor.SetEvictable(false)

	// if we can acquire token, that means we are here due to
	// abort/shutdown/timeout, attempt to acquire and terminate,
	// otherwise continue processing the request
	if call.slots.acquireSlot(s) {
		slot.Close()
		return false
	}

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

	state.UpdateState(ctx, ContainerStateBusy, call.slots)
	return true
}

// container implements drivers.ContainerTask container is the execution of a
// single container, which may run multiple functions [consecutively]. the id
// and stderr can be swapped out by new calls in the container.  input and
// output must be copied in and out. stdout is sent to stderr.
type container struct {
	id         string // contrived
	image      string
	env        map[string]string
	extensions map[string]string
	memory     uint64
	cpus       uint64
	fsSize     uint64
	tmpFsSize  uint64
	iofs       iofs
	logCfg     drivers.LoggerConfig
	close      func()

	stderr io.Writer

	udsClient http.Client

	// swapMu protects the stats swapping
	swapMu sync.Mutex
	stats  *drivers.Stats
}

// newHotContainer creates a container that can be used for multiple sequential events
func newHotContainer(ctx context.Context, call *call, cfg *Config, id string, udsWait chan error) *container {

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

	return &container{
		id:         id, // XXX we could just let docker generate ids...
		image:      call.Image,
		env:        map[string]string(call.Config),
		extensions: call.extensions,
		memory:     call.Memory,
		cpus:       uint64(call.CPUs),
		fsSize:     cfg.MaxFsSize,
		tmpFsSize:  uint64(call.TmpFsSize),
		iofs:       iofs,
		logCfg: drivers.LoggerConfig{
			URL: strings.TrimSpace(call.SyslogURL),
			Tags: []drivers.LoggerTag{
				{Name: "app_id", Value: call.AppID},
				{Name: "fn_id", Value: call.FnID},
			},
		},
		stderr: stderr,
		udsClient: http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        1,
				MaxIdleConnsPerHost: 1,
				// XXX(reed): other settings ?
				IdleConnTimeout: 1 * time.Second,
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", filepath.Join(iofs.AgentPath(), udsFilename))
				},
			},
		},
		close: func() {
			stderr.Close()
			for _, b := range bufs {
				bufPool.Put(b)
			}
			if err := iofs.Close(); err != nil {
				logger.WithError(err).Error("Error closing IOFS")
			}
		},
	}
}

func (c *container) swap(stderr io.Writer, cs *drivers.Stats) func() {
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
func (c *container) Close()                             { c.close() }
func (c *container) Image() string                      { return c.image }
func (c *container) Timeout() time.Duration             { return 0 } // context handles this
func (c *container) EnvVars() map[string]string         { return c.env }
func (c *container) Memory() uint64                     { return c.memory * 1024 * 1024 } // convert MB
func (c *container) CPUs() uint64                       { return c.cpus }
func (c *container) FsSize() uint64                     { return c.fsSize }
func (c *container) TmpFsSize() uint64                  { return c.tmpFsSize }
func (c *container) Extensions() map[string]string      { return c.extensions }
func (c *container) LoggerConfig() drivers.LoggerConfig { return c.logCfg }
func (c *container) UDSAgentPath() string               { return c.iofs.AgentPath() }
func (c *container) UDSDockerPath() string              { return c.iofs.DockerPath() }
func (c *container) UDSDockerDest() string              { return iofsDockerMountDest }

// WriteStat publishes each metric in the specified Stats structure as a histogram metric
func (c *container) WriteStat(ctx context.Context, stat drivers.Stat) {
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

// DockerAuth implements the docker.AuthConfiguration interface.
func (c *container) DockerAuth() (*docker.AuthConfiguration, error) {
	registryToken := c.extensions[RegistryToken]
	if registryToken != "" {
		return &docker.AuthConfiguration{
			RegistryToken: registryToken,
		}, nil
	}
	return nil, nil
}
