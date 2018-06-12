package agent

import (
	"bytes"
	"context"
	"io"
	"log/syslog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/agent/protocol"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/trace"
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
// TODO if a cold call times out but container is created but hasn't replied, could
// end up that the client doesn't get a reply until long after the timeout (b/c of container removal, async it?)
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
//	* manage the container lifecycle for calls (hot+cold)
//	* execute calls against containers
//	* invoke Start and End for each call appropriately
//	* check the mq for any async calls, and submit them
//
// Overview:
// Upon submission of a call, Agent will start the call's timeout timer
// immediately. If the call is hot, Agent will attempt to find an active hot
// container for that route, and if necessary launch another container. Cold
// calls will launch one container each. Cold calls will get container input
// and output directly, whereas hot calls will be able to read/write directly
// from/to a pipe in a container via Dispatch. If it's necessary to launch a
// container, first an attempt will be made to try to reserve the ram required
// while waiting for any hot 'slot' to become available [if applicable]. If
// there is an error launching the container, an error will be returned
// provided the call has not yet timed out or found another hot 'slot' to
// execute in [if applicable]. call.Start will be called immediately before
// starting a container, if cold (i.e. after pulling), or immediately before
// sending any input, if hot. call.End will be called regardless of the
// timeout timer's status if the call was executed, and that error returned may
// be returned from Submit.
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

	// Enqueue is to use the agent's sweet sweet client bindings to remotely
	// queue async tasks and should be removed from Agent interface ASAP.
	Enqueue(context.Context, *models.Call) error

	// GetAppID is to get the match of an app name to its ID
	GetAppID(ctx context.Context, appName string) (string, error)

	// GetAppByID is to get the app by ID
	GetAppByID(ctx context.Context, appID string) (*models.App, error)

	// GetRoute is to get the route by appId and path
	GetRoute(ctx context.Context, appID string, path string) (*models.Route, error)
}

type agent struct {
	cfg           AgentConfig
	da            DataAccess
	callListeners []fnext.CallListener

	driver drivers.Driver

	slotMgr *slotQueueMgr
	// track usage
	resources ResourceTracker

	// used to track running calls / safe shutdown
	shutWg       *common.WaitGroup
	shutonce     sync.Once
	callEndCount int64
}

type AgentOption func(*agent) error

// New creates an Agent that executes functions locally as Docker containers.
func New(da DataAccess, options ...AgentOption) Agent {
	a := createAgent(da, options...).(*agent)
	if !a.shutWg.AddSession(1) {
		logrus.Fatalf("cannot start agent, unable to add session")
	}
	go a.asyncDequeue() // safe shutdown can nanny this fine
	return a
}

func WithConfig(cfg *AgentConfig) AgentOption {
	return func(a *agent) error {
		a.cfg = *cfg
		return nil
	}
}

func createAgent(da DataAccess, options ...AgentOption) Agent {
	cfg, err := NewAgentConfig()
	if err != nil {
		logrus.WithError(err).Fatalf("error in agent config cfg=%+v", cfg)
	}

	a := &agent{
		cfg: *cfg,
	}

	// Allow overriding config
	for _, option := range options {
		err = option(a)
		if err != nil {
			logrus.WithError(err).Fatalf("error in agent options")
		}
	}

	logrus.Infof("agent starting cfg=%+v", a.cfg)

	// TODO: Create drivers.New(runnerConfig)
	a.driver = docker.NewDocker(drivers.Config{
		DockerNetworks:       a.cfg.DockerNetworks,
		ServerVersion:        a.cfg.MinDockerVersion,
		PreForkPoolSize:      a.cfg.PreForkPoolSize,
		PreForkImage:         a.cfg.PreForkImage,
		PreForkCmd:           a.cfg.PreForkCmd,
		PreForkUseOnce:       a.cfg.PreForkUseOnce,
		PreForkNetworks:      a.cfg.PreForkNetworks,
		MaxTmpFsInodes:       a.cfg.MaxTmpFsInodes,
		EnableReadOnlyRootFs: !a.cfg.DisableReadOnlyRootFs,
	})

	a.da = da
	a.slotMgr = NewSlotQueueMgr()
	a.resources = NewResourceTracker(&a.cfg)
	a.shutWg = common.NewWaitGroup()

	// TODO assert that agent doesn't get started for API nodes up above ?
	return a
}

func (a *agent) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	return a.da.GetAppByID(ctx, appID)
}

func (a *agent) GetAppID(ctx context.Context, appName string) (string, error) {
	return a.da.GetAppID(ctx, appName)
}

func (a *agent) GetRoute(ctx context.Context, appID string, path string) (*models.Route, error) {
	return a.da.GetRoute(ctx, appID, path)
}

// TODO shuffle this around somewhere else (maybe)
func (a *agent) Enqueue(ctx context.Context, call *models.Call) error {
	return a.da.Enqueue(ctx, call)
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

	// shutdown any db/queue resources
	// associated with DataAccess
	daErr := a.da.Close()
	if daErr != nil {
		return daErr
	}
	return err
}

func (a *agent) Submit(callI Call) error {
	if !a.shutWg.AddSession(1) {
		return models.ErrCallTimeoutServerBusy
	}

	call := callI.(*call)

	ctx := call.req.Context()
	ctx, span := trace.StartSpan(ctx, "agent_submit")
	defer span.End()

	err := a.submit(ctx, call)
	return err
}

func (a *agent) startStateTrackers(ctx context.Context, call *call) {

	if !protocol.IsStreamable(protocol.Protocol(call.Format)) {
		// For cold containers, we track the container state in call
		call.containerState = NewContainerState()
	}

	call.requestState = NewRequestState()
}

func (a *agent) endStateTrackers(ctx context.Context, call *call) {

	call.requestState.UpdateState(ctx, RequestStateDone, call.slots)

	// For cold containers, we are done with the container.
	if call.containerState != nil {
		call.containerState.UpdateState(ctx, ContainerStateDone, call.slots)
	}
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

	statsDequeueAndStart(ctx)

	// We are about to execute the function, set container Exec Deadline (call.Timeout)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(call.Timeout)*time.Second)
	defer cancel()

	// Pass this error (nil or otherwise) to end directly, to store status, etc.
	err = slot.exec(ctx, call)
	return a.handleCallEnd(ctx, call, slot, err, true)
}

func (a *agent) scheduleCallEnd(fn func()) {
	atomic.AddInt64(&a.callEndCount, 1)
	go func() {
		fn()
		atomic.AddInt64(&a.callEndCount, -1)
		a.shutWg.DoneSession()
	}()
}

func (a *agent) finalizeCallEnd(ctx context.Context, err error, isRetriable, isScheduled bool) error {
	// if scheduled in background, let scheduleCallEnd() handle
	// the shutWg group, otherwise decrement here.
	if !isScheduled {
		a.shutWg.DoneSession()
	}
	handleStatsEnd(ctx, err)
	return transformTimeout(err, isRetriable)
}

func (a *agent) handleCallEnd(ctx context.Context, call *call, slot Slot, err error, isCommitted bool) error {

	// For hot-containers, slot close is a simple channel close... No need
	// to handle it async. Execute it here ASAP
	if slot != nil && protocol.IsStreamable(protocol.Protocol(call.Format)) {
		slot.Close(ctx)
		slot = nil
	}

	// This means call was routed (executed), in order to reduce latency here
	// we perform most of these tasks in go-routine asynchronously.
	if isCommitted {
		a.scheduleCallEnd(func() {
			ctx = common.BackgroundContext(ctx)
			if slot != nil {
				slot.Close(ctx) // (no timeout)
			}
			ctx, cancel := context.WithTimeout(ctx, a.cfg.CallEndTimeout)
			call.End(ctx, err)
			cancel()
		})
		return a.finalizeCallEnd(ctx, err, false, true)
	}

	// The call did not succeed. And it is retriable. We close the slot
	// ASAP in the background if we haven't already done so (cold-container case),
	// in order to keep latency down.
	if slot != nil {
		a.scheduleCallEnd(func() {
			slot.Close(common.BackgroundContext(ctx)) // (no timeout)
		})
		return a.finalizeCallEnd(ctx, err, true, true)
	}

	return a.finalizeCallEnd(ctx, err, true, false)
}

func transformTimeout(e error, isRetriable bool) error {
	if e == context.DeadlineExceeded {
		if isRetriable {
			return models.ErrCallTimeoutServerBusy
		}
		return models.ErrCallTimeout
	} else if e == CapacityFull {
		return models.ErrCallTimeoutServerBusy
	}
	return e
}

// handleStatsDequeue handles stats for dequeuing for early exit (getSlot or Start)
// cases. Only timeouts can be a simple dequeue while other cases are actual errors.
func handleStatsDequeue(ctx context.Context, err error) {
	if err == context.DeadlineExceeded {
		statsDequeue(ctx)
		statsTooBusy(ctx)
	} else {
		statsDequeueAndFail(ctx)
		statsErrors(ctx)
	}
}

// handleStatsEnd handles stats for after a call is ran, depending on error.
func handleStatsEnd(ctx context.Context, err error) {
	if err == nil {
		// decrement running count, increment completed count
		statsComplete(ctx)
	} else {
		// decrement running count, increment failed count
		statsFailed(ctx)
		// increment the timeout or errors count, as appropriate
		if err == context.DeadlineExceeded {
			statsTimedout(ctx)
		} else {
			statsErrors(ctx)
		}
	}
}

// getSlot returns a Slot (or error) for the request to run. Depending on hot/cold
// request type, this may launch a new container or wait for other containers to become idle
// or it may wait for resources to become available to launch a new container.
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

	// first check any excess case of call.End() stacking.
	if atomic.LoadInt64(&a.callEndCount) >= int64(a.cfg.MaxCallEndStacking) {
		return nil, context.DeadlineExceeded
	}

	if protocol.IsStreamable(protocol.Protocol(call.Format)) {
		// For hot requests, we use a long lived slot queue, which we use to manage hot containers
		var isNew bool
		call.slots, isNew = a.slotMgr.getSlotQueue(call)
		call.requestState.UpdateState(ctx, RequestStateWait, call.slots)
		if isNew {
			go a.hotLauncher(ctx, call)
		}
		s, err := a.waitHot(ctx, call)
		return s, err
	}

	call.requestState.UpdateState(ctx, RequestStateWait, call.slots)
	return a.launchCold(ctx, call)
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
	logger.WithField("launcher_timeout", timeout).Info("Hot function launcher starting")

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
				logger.Info("Hot function launcher timed out")
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
	isAsync := call.Type == models.TypeAsync
	isNB := a.cfg.EnableNBResourceTracker
	isNeeded := isNewContainerNeeded(&curStats)
	common.Logger(ctx).WithFields(logrus.Fields{"currentStats": curStats, "isNeeded": isNeeded}).Debug("Hot function launcher stats")
	if !isNeeded {
		return
	}

	state := NewContainerState()
	state.UpdateState(ctx, ContainerStateWait, call.slots)

	common.Logger(ctx).WithFields(logrus.Fields{"currentStats": call.slots.getStats(), "isNeeded": isNeeded}).Info("Hot function launcher starting hot container")

	mem := call.Memory + uint64(call.TmpFsSize)

	// WARNING: Tricky flow below. We are here because: isNeeded is set,
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
	// Non-blocking mode only applies to cpu+mem, and if isNeeded decided that we do not
	// need to start a new container, then waiters will wait.
	select {
	case tok := <-a.resources.GetResourceToken(ctx, mem, uint64(call.CPUs), isAsync, isNB):
		if tok != nil && tok.Error() != nil {
			tryNotify(notifyChan, tok.Error())
		} else if a.shutWg.AddSession(1) {
			go func() {
				// NOTE: runHot will not inherit the timeout from ctx (ignore timings)
				a.runHot(ctx, call, tok, state)
				a.shutWg.DoneSession()
			}()
			return
		}
		if tok != nil {
			tok.Close()
		}
	case <-ctx.Done(): // timeout
	case <-a.shutWg.Closer(): // server shutdown
	}

	state.UpdateState(ctx, ContainerStateDone, call.slots)
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
					s.slot.Close(ctx)
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

// launchCold waits for necessary resources to launch a new container, then
// returns the slot for that new container to run the request on.
func (a *agent) launchCold(ctx context.Context, call *call) (Slot, error) {
	isAsync := call.Type == models.TypeAsync
	isNB := a.cfg.EnableNBResourceTracker

	ch := make(chan Slot)

	ctx, span := trace.StartSpan(ctx, "agent_launch_cold")
	defer span.End()

	call.containerState.UpdateState(ctx, ContainerStateWait, call.slots)

	mem := call.Memory + uint64(call.TmpFsSize)

	select {
	case tok := <-a.resources.GetResourceToken(ctx, mem, uint64(call.CPUs), isAsync, isNB):
		if tok.Error() != nil {
			return nil, tok.Error()
		}

		go a.prepCold(ctx, call, tok, ch)
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// wait for launch err or a slot to open up
	select {
	case s := <-ch:
		if s.Error() != nil {
			s.Close(ctx)
			return nil, s.Error()
		}
		return s, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// implements Slot
type coldSlot struct {
	cookie   drivers.Cookie
	tok      ResourceToken
	fatalErr error
}

func (s *coldSlot) Error() error {
	return s.fatalErr
}

func (s *coldSlot) exec(ctx context.Context, call *call) error {
	ctx, span := trace.StartSpan(ctx, "agent_cold_exec")
	defer span.End()

	call.requestState.UpdateState(ctx, RequestStateExec, call.slots)
	call.containerState.UpdateState(ctx, ContainerStateBusy, call.slots)

	waiter, err := s.cookie.Run(ctx)
	if err != nil {
		return err
	}

	res := waiter.Wait(ctx)
	if res.Error() != nil {
		// check for call error (oom/exit) and beam it up
		return res.Error()
	}

	// nil or timed out
	return ctx.Err()
}

func (s *coldSlot) Close(ctx context.Context) error {
	if s.cookie != nil {
		s.cookie.Close(ctx)
	}
	if s.tok != nil {
		s.tok.Close()
	}
	return nil
}

// implements Slot
type hotSlot struct {
	done          chan struct{} // signal we are done with slot
	errC          <-chan error  // container error
	container     *container    // TODO mask this
	maxRespSize   uint64        // TODO boo.
	fatalErr      error
	containerSpan trace.SpanContext
}

func (s *hotSlot) Close(ctx context.Context) error {
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

	// swap in fresh pipes & stat accumulator to not interlace with other calls that used this slot [and timed out]
	stdinRead, stdinWrite := io.Pipe()
	stdoutRead, stdoutWritePipe := io.Pipe()
	defer stdinRead.Close()
	defer stdoutWritePipe.Close()

	// NOTE: stderr is limited separately (though line writer is vulnerable to attack?)
	// limit the bytes allowed to be written to the stdout pipe, which handles any
	// buffering overflows (json to a string, http to a buffer, etc)
	stdoutWrite := common.NewClampWriter(stdoutWritePipe, s.maxRespSize, models.ErrFunctionResponseTooBig)

	// get our own syslogger with THIS call id (cheap), using the container's already open syslog conns (expensive)
	// TODO? we can basically just do this whether there are conns or not, this is relatively cheap (despite appearances)
	buf1 := bufPool.Get().(*bytes.Buffer)
	buf2 := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf1)
	defer bufPool.Put(buf2)

	sw := newSyslogWriter(call.ID, call.Path, call.AppID, syslog.LOG_ERR, s.container.syslogConns, buf1)
	var syslog io.WriteCloser = &nopCloser{sw}
	syslog = newLineWriterWithBuffer(buf2, syslog)
	defer syslog.Close()                            // close syslogger from here, but NOT the call log stderr OR conns
	stderr := multiWriteCloser{call.stderr, syslog} // use multiWriteCloser for its error ignoring properties

	proto := protocol.New(protocol.Protocol(call.Format), stdinWrite, stdoutRead)
	swapBack := s.container.swap(stdinRead, stdoutWrite, stderr, &call.Stats)
	defer swapBack() // NOTE: it's important this runs before the pipes are closed.

	errApp := make(chan error, 1)
	go func() {
		ci := protocol.NewCallInfo(call.IsCloudEvent, call.Call, call.req.WithContext(ctx))
		errApp <- proto.Dispatch(ctx, ci, call.w)
	}()

	select {
	case err := <-s.errC: // error from container
		s.trySetError(err)
		return err
	case err := <-errApp: // from dispatch
		if err != nil {
			if models.IsAPIError(err) {
				s.trySetError(err)
			} else if err == protocol.ErrExcessData {
				s.trySetError(err)
				// suppress excess data error, but do shutdown the container
				return nil
			}
		}
		return err
	case <-ctx.Done(): // call timeout
		s.trySetError(ctx.Err())
		return ctx.Err()
	}
}

func (a *agent) prepCold(ctx context.Context, call *call, tok ResourceToken, ch chan Slot) {
	ctx, span := trace.StartSpan(ctx, "agent_prep_cold")
	defer span.End()

	call.containerState.UpdateState(ctx, ContainerStateStart, call.slots)

	deadline := time.Now().Add(time.Duration(call.Timeout) * time.Second)

	// add Fn-specific information to the config to shove everything into env vars for cold
	call.Config["FN_DEADLINE"] = strfmt.DateTime(deadline).String()
	call.Config["FN_METHOD"] = call.Model().Method
	call.Config["FN_REQUEST_URL"] = call.Model().URL
	call.Config["FN_CALL_ID"] = call.Model().ID

	// User headers are prefixed with FN_HEADER and shoved in the env vars too
	for k, v := range call.Headers {
		k = "FN_HEADER_" + k
		call.Config[k] = strings.Join(v, ", ")
	}

	container := &container{
		id:      id.New().String(), // XXX we could just let docker generate ids...
		image:   call.Image,
		env:     map[string]string(call.Config),
		memory:  call.Memory,
		cpus:    uint64(call.CPUs),
		fsSize:  a.cfg.MaxFsSize,
		timeout: time.Duration(call.Timeout) * time.Second, // this is unnecessary, but in case removal fails...
		stdin:   call.req.Body,
		stdout:  common.NewClampWriter(call.w, a.cfg.MaxResponseSize, models.ErrFunctionResponseTooBig),
		stderr:  call.stderr,
		stats:   &call.Stats,
	}

	// pull & create container before we return a slot, so as to be friendly
	// about timing out if this takes a while...
	cookie, err := a.driver.Prepare(ctx, container)

	call.containerState.UpdateState(ctx, ContainerStateIdle, call.slots)

	slot := &coldSlot{cookie, tok, err}
	select {
	case ch <- slot:
	case <-ctx.Done():
		slot.Close(ctx)
	}
}

func (a *agent) runHot(ctx context.Context, call *call, tok ResourceToken, state ContainerState) {
	// IMPORTANT: get a context that has a child span / logger but NO timeout
	// TODO this is a 'FollowsFrom'
	ctx = common.BackgroundContext(ctx)
	ctx, span := trace.StartSpan(ctx, "agent_run_hot")
	defer span.End()
	defer tok.Close() // IMPORTANT: this MUST get called

	state.UpdateState(ctx, ContainerStateStart, call.slots)
	defer state.UpdateState(ctx, ContainerStateDone, call.slots)

	container, closer := NewHotContainer(ctx, call, &a.cfg)
	defer closer()

	logger := logrus.WithFields(logrus.Fields{"id": container.id, "app_id": call.AppID, "route": call.Path, "image": call.Image, "memory": call.Memory, "cpus": call.CPUs, "format": call.Format, "idle_timeout": call.IdleTimeout})
	ctx = common.WithLogger(ctx, logger)

	cookie, err := a.driver.Prepare(ctx, container)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
		return
	}
	defer cookie.Close(ctx) // NOTE ensure this ctx doesn't time out

	waiter, err := cookie.Run(ctx)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), fatalErr: err})
		return
	}

	// container is running
	state.UpdateState(ctx, ContainerStateIdle, call.slots)

	// buffered, in case someone has slot when waiter returns but isn't yet listening
	errC := make(chan error, 1)

	ctx, shutdownContainer := context.WithCancel(ctx)
	defer shutdownContainer() // close this if our waiter returns, to call off slots
	go func() {
		defer shutdownContainer() // also close if we get an agent shutdown / idle timeout

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
				maxRespSize:   a.cfg.MaxResponseSize,
				containerSpan: trace.FromContext(ctx).SpanContext(),
			}
			if !a.runHotReq(ctx, call, state, logger, cookie, slot) {
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

	logger.WithError(res.Error()).Info("hot function terminated")
}

// runHotReq enqueues a free slot to slot queue manager and watches various timers and the consumer until
// the slot is consumed. A return value of false means, the container should shutdown and no subsequent
// calls should be made to this function.
func (a *agent) runHotReq(ctx context.Context, call *call, state ContainerState, logger logrus.FieldLogger, cookie drivers.Cookie, slot *hotSlot) bool {

	var err error
	isFrozen := false

	freezeTimer := time.NewTimer(a.cfg.FreezeIdle)
	idleTimer := time.NewTimer(time.Duration(call.IdleTimeout) * time.Second)
	ejectTicker := time.NewTicker(a.cfg.EjectIdle)

	defer freezeTimer.Stop()
	defer idleTimer.Stop()
	defer ejectTicker.Stop()

	// log if any error is encountered
	defer func() {
		if err != nil {
			logger.WithError(err).Error("hot function failure")
		}
	}()

	// if an immediate freeze is requested, freeze first before enqueuing at all.
	if a.cfg.FreezeIdle == time.Duration(0) && !isFrozen {
		err = cookie.Freeze(ctx)
		if err != nil {
			return false
		}
		isFrozen = true
	}

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
			}
			continue
		case <-ejectTicker.C:
			// if someone is waiting for resource in our slot queue, we must not terminate,
			// otherwise, see if other slot queues have resource waiters that are blocked.
			stats := call.slots.getStats()
			if stats.containerStates[ContainerStateWait] > 0 ||
				a.resources.GetResourceTokenWaiterCount() <= 0 {
				continue
			}
			logger.Debug("attempting hot function eject")
		}
		break
	}

	// if we can acquire token, that means we are here due to
	// abort/shutdown/timeout, attempt to acquire and terminate,
	// otherwise continue processing the request
	if call.slots.acquireSlot(s) {
		slot.Close(ctx)
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
	id        string // contrived
	image     string
	env       map[string]string
	memory    uint64
	cpus      uint64
	fsSize    uint64
	tmpFsSize uint64
	timeout   time.Duration // cold only (superfluous, but in case)

	stdin       io.Reader
	stdout      io.Writer
	stderr      io.Writer
	syslogConns io.WriteCloser

	// swapMu protects the stats swapping
	swapMu sync.Mutex
	stats  *drivers.Stats
}

func NewHotContainer(ctx context.Context, call *call, cfg *AgentConfig) (*container, func()) {
	// if freezer is enabled, be consistent with freezer behavior and
	// block stdout and stderr between calls.
	isBlockIdleIO := MaxDisabledMsecs != cfg.FreezeIdle

	id := id.New().String()

	stdin := common.NewGhostReader()
	stderr := common.NewGhostWriter()
	stdout := common.NewGhostWriter()

	// these are only the conns, this doesn't write the syslog format (since it will change between calls)
	syslogConns, err := syslogConns(ctx, call.SyslogURL)
	if err != nil {
		// TODO we could write this to between stderr but between stderr doesn't go to user either. kill me.
		common.Logger(ctx).WithError(err).WithFields(logrus.Fields{"app_id": call.AppID, "path": call.Path, "image": call.Image, "container_id": id}).Error("error dialing syslog urls")
	}

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
		// stderr -> line writer -> syslog -> []conns

		// TODO(reed): I guess this is worth it
		// TODO(reed): there's a bug here where the between writers could have
		// bytes in there, get swapped for real stdout/stderr, come back and write
		// bytes in and the bytes are [really] stale. I played with fixing this
		// and mostly came to the conclusion that life is meaningless.
		buf1 := bufPool.Get().(*bytes.Buffer)
		buf2 := bufPool.Get().(*bytes.Buffer)
		buf3 := bufPool.Get().(*bytes.Buffer)
		buf4 := bufPool.Get().(*bytes.Buffer)
		bufs = []*bytes.Buffer{buf1, buf2, buf3, buf4}

		// stdout = LOG_INFO, stderr = LOG_ERR -- ONLY for the between writers, normal stdout is a response
		so := newSyslogWriter(call.ID, call.Path, call.AppID, syslog.LOG_INFO, syslogConns, buf1)
		se := newSyslogWriter(call.ID, call.Path, call.AppID, syslog.LOG_ERR, syslogConns, buf2)

		// use multiWriteCloser since it ignores errors (io.MultiWriter does not)
		soc := multiWriteCloser{&nopCloser{so}, &nopCloser{&logWriter{
			logrus.WithFields(logrus.Fields{"tag": "stdout", "app_id": call.AppID, "path": call.Path, "image": call.Image, "container_id": id}),
		}}}
		sec := multiWriteCloser{&nopCloser{se}, &nopCloser{&logWriter{
			logrus.WithFields(logrus.Fields{"tag": "stderr", "app_id": call.AppID, "path": call.Path, "image": call.Image, "container_id": id}),
		}}}

		stdout.Swap(newLineWriterWithBuffer(buf4, soc))
		stderr.Swap(newLineWriterWithBuffer(buf3, sec))
	}

	return &container{
			id:          id, // XXX we could just let docker generate ids...
			image:       call.Image,
			env:         map[string]string(call.Config),
			memory:      call.Memory,
			cpus:        uint64(call.CPUs),
			fsSize:      cfg.MaxFsSize,
			tmpFsSize:   uint64(call.TmpFsSize),
			stdin:       stdin,
			stdout:      stdout,
			stderr:      stderr,
			syslogConns: syslogConns,
		}, func() {
			stdin.Close()
			stderr.Close()
			stdout.Close()
			for _, b := range bufs {
				bufPool.Put(b)
			}
			syslogConns.Close()
		}
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

func (c *container) Id() string                     { return c.id }
func (c *container) Command() string                { return "" }
func (c *container) Input() io.Reader               { return c.stdin }
func (c *container) Logger() (io.Writer, io.Writer) { return c.stdout, c.stderr }
func (c *container) Volumes() [][2]string           { return nil }
func (c *container) WorkDir() string                { return "" }
func (c *container) Close()                         {}
func (c *container) Image() string                  { return c.image }
func (c *container) Timeout() time.Duration         { return c.timeout }
func (c *container) EnvVars() map[string]string     { return c.env }
func (c *container) Memory() uint64                 { return c.memory * 1024 * 1024 } // convert MB
func (c *container) CPUs() uint64                   { return c.cpus }
func (c *container) FsSize() uint64                 { return c.fsSize }
func (c *container) TmpFsSize() uint64              { return c.tmpFsSize }

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

//func (c *container) DockerAuth() (docker.AuthConfiguration, error) {
// Implementing the docker.AuthConfiguration interface.
// TODO per call could implement this stored somewhere (vs. configured on host)
//}
