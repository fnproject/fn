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
	shutWg              *common.WaitGroup
	shutonce            sync.Once
	disableAsyncDequeue bool

	callOverrider CallOverrider
	// deferred actions to call at end of initialisation
	onStartup []func()
}

// Option configures an agent at startup
type Option func(*agent) error

// RegistryToken is a reserved call extensions key to pass registry token
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
			logrus.WithError(err).Fatalf("error in agent options")
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
		if !a.shutWg.AddSession(1) {
			logrus.Fatalf("cannot start agent, unable to add session")
		}
		a.addStartup(func() {
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
		EnableTini:           !cfg.DisableTini,
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
		}
	} else {
		statsDequeue(ctx)
		if err == CapacityFull || err == context.DeadlineExceeded {
			statsTooBusy(ctx)
			return models.ErrCallTimeoutServerBusy
		}
	}

	if err == context.DeadlineExceeded {
		statsTimedout(ctx)
		return models.ErrCallTimeout
	} else if err == context.Canceled {
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
	if isNew {
		go a.hotLauncher(ctx, call)
	}
	s, err := a.waitHot(ctx, call)
	return s, err
}

// hotLauncher is spawned in a go routine for each slot queue to monitor stats and launch hot
// containers if needed. Upon shutdown or activity timeout, hotLauncher exits and during exit,
// it destroys the slot queue.
func (a *agent) hotLauncher(ctx context.Context, call *call) {
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

	var notifyChan chan error

	for {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		a.checkLaunch(ctx, call, notifyChan)
		notifyChan = nil

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
		case notifyChan = <-call.slots.signaller:
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

func (a *agent) checkLaunch(ctx context.Context, call *call, notifyChan chan error) {
	curStats := call.slots.getStats()
	isNB := a.cfg.EnableNBResourceTracker
	if !isNewContainerNeeded(&curStats) {
		return
	}

	state := NewContainerState()
	state.UpdateState(ctx, ContainerStateWait, call.slots)

	mem := call.Memory + uint64(call.TmpFsSize)

	var notifyChans []chan struct{}

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
	case tok := <-a.resources.GetResourceToken(ctx, mem, call.CPUs, isNB):
		if tok != nil && tok.Error() != nil {
			if tok.Error() != CapacityFull {
				tryNotify(notifyChan, tok.Error())
			} else {
				notifyChans = a.evictor.PerformEviction(call.slotHashId, mem, uint64(call.CPUs))
				if len(notifyChans) == 0 {
					tryNotify(notifyChan, tok.Error())
				}
			}
		} else if a.shutWg.AddSession(1) {
			go func() {
				// NOTE: runHot will not inherit the timeout from ctx (ignore timings)
				a.runHot(ctx, call, tok, state)
				a.shutWg.DoneSession()
			}()
			// early return (do not allow container state to switch to ContainerStateDone)
			return
		}
		if tok != nil {
			statsUtilization(ctx, a.resources.GetUtilization())
			tok.Close()
		}
		// Request routines are polling us with this a.cfg.HotPoll frequency. We can use this
		// same timer to assume that we waited for cpu/mem long enough. Let's try to evict an
		// idle container.
	case <-time.After(a.cfg.HotPoll):
		notifyChans = a.evictor.PerformEviction(call.slotHashId, mem, uint64(call.CPUs))
	case <-ctx.Done(): // timeout
	case <-a.shutWg.Closer(): // server shutdown
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
func (a *agent) waitHot(ctx context.Context, call *call) (Slot, error) {
	ctx, span := trace.StartSpan(ctx, "agent_wait_hot")
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // shut down dequeuer if we grab a slot

	ch := call.slots.startDequeuer(ctx)

	notifyChan := make(chan error)

	// 1) if we can get a slot immediately, grab it.
	// 2) if we don't, send a signaller every x msecs until we do.

	sleep := 1 * time.Microsecond // pad, so time.After doesn't send immediately
	for {
		select {
		case err := <-notifyChan:
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
		case call.slots.signaller <- notifyChan:
		default:
		}
	}
}

// implements Slot
type hotSlot struct {
	done          chan struct{} // signal we are done with slot
	errC          <-chan error  // container error
	container     *container    // TODO mask this
	cfg           *Config
	udsClient     http.Client
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

	errApp := s.dispatch(ctx, call)

	select {
	case err := <-s.errC: // error from container
		s.trySetError(err)
		return err
	case err := <-errApp: // from dispatch
		if err != nil {
			if models.IsAPIError(err) {
				s.trySetError(err)
			}
		}
		return err
	case <-ctx.Done(): // call timeout
		s.trySetError(ctx.Err())
		return ctx.Err()
	}
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

func callToHTTPRequest(ctx context.Context, call *call) *http.Request {
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

	//req.Header.Set("FN_DEADLINE", ci.Deadline().String())
	// TODO(occ) : fix compatidupes when FDKs are updated
	req.Header.Set("Fn-Call-Id", call.ID)
	req.Header.Set("FN_CALL_ID", call.ID)
	deadline, ok := ctx.Deadline()
	if ok {
		deadlineStr := deadline.Format(time.RFC3339)
		req.Header.Set("Fn-Deadline", deadlineStr)
		req.Header.Set("FN_DEADLINE", deadlineStr)
	}

	return req
}

func (s *hotSlot) dispatch(ctx context.Context, call *call) chan error {
	// TODO we can't trust that resp.Write doesn't timeout, even if the http
	// client should respect the request context (right?) so we still need this (right?)
	errApp := make(chan error, 1)

	go func() {
		ctx, span := trace.StartSpan(ctx, "agent_dispatch_httpstream")
		defer span.End()

		// TODO it's possible we can get rid of this (after getting rid of logs API) - may need for call id/debug mode still
		// TODO there's a timeout race for swapping this back if the container doesn't get killed for timing out, and don't you forget it
		swapBack := s.container.swap(nil, call.stderr, call.stderr, &call.Stats)
		defer swapBack()

		req := callToHTTPRequest(ctx, call)
		resp, err := s.udsClient.Do(req)
		if err != nil {
			common.Logger(ctx).WithError(err).Error("Got error from UDS socket")
			errApp <- models.NewAPIError(http.StatusBadGateway, errors.New("error receiving function response"))
			return
		}
		common.Logger(ctx).WithField("resp", resp).Debug("Got resp from UDS socket")

		// if ctx is canceled/timedout, then we close the body to unlock writeResp() below
		defer resp.Body.Close()

		ioErrChan := make(chan error, 1)
		go func() {
			ioErrChan <- writeResp(s.cfg.MaxResponseSize, resp, call.w)
		}()

		select {
		case ioErr := <-ioErrChan:
			errApp <- ioErr
		case <-ctx.Done():
			errApp <- ctx.Err()
		}
	}()
	return errApp
}

func writeResp(max uint64, resp *http.Response, w io.Writer) error {
	rw, ok := w.(http.ResponseWriter)
	if !ok {
		w = common.NewClampWriter(w, max, models.ErrFunctionResponseTooBig)
		return resp.Write(w)
	}

	rw = newSizerRespWriter(max, rw)

	// if we're writing directly to the response writer, we need to set headers
	// and status code, and only copy the body. resp.Write would copy a full
	// http request into the response body (not what we want).

	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}
	if resp.StatusCode > 0 {
		rw.WriteHeader(resp.StatusCode)
	}
	_, err := io.Copy(rw, resp.Body)
	return err
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

func (a *agent) runHot(ctx context.Context, call *call, tok ResourceToken, state ContainerState) {
	// IMPORTANT: get a context that has a child span / logger but NO timeout
	// TODO this is a 'FollowsFrom'
	ctx = common.BackgroundContext(ctx)
	ctx, span := trace.StartSpan(ctx, "agent_run_hot")
	defer span.End()

	// IMPORTANT: evict token is deleted *after* resource token in defer statements below.
	// This ordering allows resource token to be freed first, which means once evict token
	// is deleted, eviction is considered to be completed.
	evictor := a.evictor.CreateEvictToken(call.slotHashId, call.Memory+uint64(call.TmpFsSize), uint64(call.CPUs))
	defer a.evictor.DeleteEvictToken(evictor)

	statsUtilization(ctx, a.resources.GetUtilization())
	defer func() {
		statsUtilization(ctx, a.resources.GetUtilization())
	}()

	defer tok.Close() // IMPORTANT: this MUST get called

	state.UpdateState(ctx, ContainerStateStart, call.slots)
	defer state.UpdateState(ctx, ContainerStateDone, call.slots)

	container, err := newHotContainer(ctx, call, &a.cfg)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
		return
	}
	defer container.Close()

	udsAwait := make(chan error)
	// start our listener before starting the container, so we don't miss the pretty things whispered in our ears
	go inotifyUDS(ctx, container.UDSAgentPath(), udsAwait)

	udsClient := http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        1,
			MaxIdleConnsPerHost: 1,
			// XXX(reed): other settings ?
			IdleConnTimeout: 1 * time.Second,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", filepath.Join(container.UDSAgentPath(), udsFilename))
			},
		},
	}

	logger := logrus.WithFields(logrus.Fields{"id": container.id, "app_id": call.AppID, "fn_id": call.FnID, "image": call.Image, "memory": call.Memory, "cpus": call.CPUs, "idle_timeout": call.IdleTimeout})
	ctx = common.WithLogger(ctx, logger)

	cookie, err := a.driver.CreateCookie(ctx, container)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
		return
	}

	defer cookie.Close(ctx)

	err = a.driver.PrepareCookie(ctx, cookie)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
		return
	}

	waiter, err := cookie.Run(ctx)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
		return
	}

	// buffered, in case someone has slot when waiter returns but isn't yet listening
	errC := make(chan error, 1)

	ctx, shutdownContainer := context.WithCancel(ctx)
	defer shutdownContainer() // close this if our waiter returns, to call off slots
	go func() {
		defer shutdownContainer() // also close if we get an agent shutdown / idle timeout

		// now we wait for the socket to be created before handing out any slots, need this
		// here in case the container dies before making the sock we need to bail
		select {
		case err := <-udsAwait: // XXX(reed): need to leave a note about pairing ctx here?
			// sends a nil error if all is good, we can proceed...
			if err != nil {
				call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
				return
			}

		case <-ctx.Done():
			call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: ctx.Err()})
			return
		}

		for {
			select { // make sure everything is up before trying to send slot
			case <-ctx.Done(): // container shutdown
				return
			case <-a.shutWg.Closer(): // server shutdown
				return
			default: // ok
			}

			slot := &hotSlot{
				done:          make(chan struct{}),
				errC:          errC,
				container:     container,
				cfg:           &a.cfg,
				udsClient:     udsClient,
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

	res := waiter.Wait(ctx)
	if res.Error() != nil {
		errC <- res.Error() // TODO: race condition, no guaranteed delivery fix this...
	}
	if res.Error() != context.Canceled {
		logger.WithError(res.Error()).Info("hot function terminated")
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
func inotifyUDS(ctx context.Context, iofsDir string, awaitUDS chan<- error) {
	// XXX(reed): I forgot how to plumb channels temporarily forgive me for this sin (inotify will timeout, this is just bad programming)
	err := inotifyAwait(ctx, iofsDir)
	if err == nil {
		err = checkSocketDestination(filepath.Join(iofsDir, udsFilename))
	}
	select {
	case awaitUDS <- err:
	case <-ctx.Done():
	}
}

func inotifyAwait(ctx context.Context, iofsDir string) error {
	ctx, span := trace.StartSpan(ctx, "inotify_await")
	defer span.End()

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error getting fsnotify watcher: %v", err)
	}
	defer func() {
		if err := fsWatcher.Close(); err != nil {
			common.Logger(ctx).WithError(err).Error("Failed to close inotify watcher")
		}
	}()

	err = fsWatcher.Add(iofsDir)
	if err != nil {
		return fmt.Errorf("error adding iofs dir to fswatcher: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			// XXX(reed): damn it would sure be nice to tell users they didn't make a uds and that's why it timed out
			return ctx.Err()
		case err := <-fsWatcher.Errors:
			return fmt.Errorf("error watching for iofs: %v", err)
		case event := <-fsWatcher.Events:
			common.Logger(ctx).WithField("event", event).Debug("fsnotify event")
			if event.Op&fsnotify.Create == fsnotify.Create && event.Name == filepath.Join(iofsDir, udsFilename) {

				// wait until the socket file is created by the container
				return nil
			}
		}
	}
}

// runHotReq enqueues a free slot to slot queue manager and watches various timers and the consumer until
// the slot is consumed. A return value of false means, the container should shutdown and no subsequent
// calls should be made to this function.
func (a *agent) runHotReq(ctx context.Context, call *call, state ContainerState, logger logrus.FieldLogger, cookie drivers.Cookie, slot *hotSlot, evictor *EvictToken) bool {

	var err error
	isFrozen := false
	isEvictEvent := false

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
		case <-a.shutWg.Closer(): // server shutdown
		case <-idleTimer.C:
		case <-freezeTimer.C:
			if !isFrozen {
				err = cookie.Freeze(ctx)
				if err != nil {
					return false
				}
				isFrozen = true
				state.UpdateState(ctx, ContainerStatePaused, call.slots)
			}
			continue
		case <-evictor.C:
			logger.Debug("attempting hot function eviction")
			isEvictEvent = true
		}
		break
	}

	evictor.SetEvictable(false)

	// if we can acquire token, that means we are here due to
	// abort/shutdown/timeout, attempt to acquire and terminate,
	// otherwise continue processing the request
	if call.slots.acquireSlot(s) {
		slot.Close()
		if isEvictEvent {
			statsContainerEvicted(ctx)
		}
		return false
	}

	// In case, timer/acquireSlot failure landed us here, make
	// sure to unfreeze.
	if isFrozen {
		err = cookie.Unfreeze(ctx)
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
// output must be copied in and out.
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

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	// swapMu protects the stats swapping
	swapMu sync.Mutex
	stats  *drivers.Stats
}

//newHotContainer creates a container that can be used for multiple sequential events
func newHotContainer(ctx context.Context, call *call, cfg *Config) (*container, error) {
	// if freezer is enabled, be consistent with freezer behavior and
	// block stdout and stderr between calls.
	isBlockIdleIO := MaxMsDisabled != cfg.FreezeIdle

	id := id.New().String()

	stdin := common.NewGhostReader()
	stderr := common.NewGhostWriter()
	stdout := common.NewGhostWriter()

	// for use if no freezer (or we ever make up our minds)
	var bufs []*bytes.Buffer

	// when not processing a request, do we block IO?
	if !isBlockIdleIO {
		// IMPORTANT: we are not operating on a TTY allocated container. This means, stderr and stdout are multiplexed
		// from the same stream internally via docker using a multiplexing protocol. Therefore, stderr/stdout *BOTH*
		// have to be read or *BOTH* blocked consistently. In other words, we cannot block one and continue
		// reading from the other one without risking head-of-line blocking.

		// wrap the syslog and debug loggers in the same (respective) line writer
		// syslog complete chain for this (from top):
		// stderr -> line writer

		// TODO(reed): I guess this is worth it
		// TODO(reed): there's a bug here where the between writers could have
		// bytes in there, get swapped for real stdout/stderr, come back and write
		// bytes in and the bytes are [really] stale. I played with fixing this
		// and mostly came to the conclusion that life is meaningless.
		buf1 := bufPool.Get().(*bytes.Buffer)
		buf2 := bufPool.Get().(*bytes.Buffer)
		bufs = []*bytes.Buffer{buf1, buf2}

		soc := &nopCloser{&logWriter{
			logrus.WithFields(logrus.Fields{"tag": "stdout", "app_id": call.AppID, "fn_id": call.FnID, "image": call.Image, "container_id": id}),
		}}
		sec := &nopCloser{&logWriter{
			logrus.WithFields(logrus.Fields{"tag": "stderr", "app_id": call.AppID, "fn_id": call.FnID, "image": call.Image, "container_id": id}),
		}}

		stdout.Swap(newLineWriterWithBuffer(buf1, soc))
		stderr.Swap(newLineWriterWithBuffer(buf2, sec))
	}

	var iofs iofs
	var err error
	// XXX(reed): we should also point stdout to stderr, and not have stdin
	if cfg.IOFSEnableTmpfs {
		iofs, err = newTmpfsIOFS(ctx, cfg)
	} else {
		iofs, err = newDirectoryIOFS(ctx, cfg)
	}

	if err != nil {
		return nil, err
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
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		close: func() {
			stdin.Close()
			stderr.Close()
			stdout.Close()
			for _, b := range bufs {
				bufPool.Put(b)
			}
			// iofs.Close MUST be called here or we will leak directories and/or tmpfs mounts!
			if err = iofs.Close(); err != nil {
				// Note: This is logged with the context of the container creation
				common.Logger(ctx).WithError(err).Error("Error closing IOFS")
			}
		},
	}, nil
}

func (c *container) swap(stdin io.Reader, stdout, stderr io.Writer, cs *drivers.Stats) func() {
	// if tests don't catch this, then fuck me
	ostdin := c.stdin.(common.GhostReader).Swap(stdin)
	ostdout := c.stdout.(common.GhostWriter).Swap(stdout)
	ostderr := c.stderr.(common.GhostWriter).Swap(stderr)

	c.swapMu.Lock()
	ocs := c.stats
	c.stats = cs
	c.swapMu.Unlock()

	return func() {
		c.stdin.(common.GhostReader).Swap(ostdin)
		c.stdout.(common.GhostWriter).Swap(ostdout)
		c.stderr.(common.GhostWriter).Swap(ostderr)
		c.swapMu.Lock()
		c.stats = ocs
		c.swapMu.Unlock()
	}
}

func (c *container) Id() string                         { return c.id }
func (c *container) Command() string                    { return "" }
func (c *container) Input() io.Reader                   { return c.stdin }
func (c *container) Logger() (io.Writer, io.Writer)     { return c.stdout, c.stderr }
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
