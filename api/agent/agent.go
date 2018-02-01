package agent

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/agent/protocol"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/go-openapi/strfmt"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// TODO we should prob store async calls in db immediately since we're returning id (will 404 until post-execution)
// TODO async calls need to add route.Headers as well
// TODO handle timeouts / no response in sync & async (sync is json+503 atm, not 504, async is empty log+status)
// see also: server/runner.go wrapping the response writer there, but need to handle async too (push down?)
// TODO storing logs / call can push call over the timeout
// TODO if we don't cap the number of any one container we could get into a situation
// where the machine is full but all the containers are idle up to the idle timeout. meh.
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
//
// Agent exposes an api to create calls from various parameters and then submit
// those calls, it also exposes a 'safe' shutdown mechanism via its Close method.
// Agent has a few roles:
// * manage the memory pool for a given server
// * manage the container lifecycle for calls (hot+cold)
// * execute calls against containers
// * invoke Start and End for each call appropriately
// * check the mq for any async calls, and submit them
//
// overview:
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
	// Close is not safe to be called from multiple threads.
	io.Closer

	// Stats should be burned at the stake. adding so as to not ruffle feathers.
	// TODO this should be derived from our metrics
	Stats() Stats

	// Return the http.Handler used to handle Prometheus metric requests
	PromHandler() http.Handler
	AddCallListener(fnext.CallListener)

	// Enqueue is to use the agent's sweet sweet client bindings to remotely
	// queue async tasks and should be removed from Agent interface ASAP.
	Enqueue(context.Context, *models.Call) error
}

type agent struct {
	da            DataAccess
	callListeners []fnext.CallListener

	driver drivers.Driver

	slotMgr *slotQueueMgr
	// track usage
	resources ResourceTracker

	// used to track running calls / safe shutdown
	wg       sync.WaitGroup // TODO rename
	shutonce sync.Once
	shutdown chan struct{}

	stats // TODO kill me

	// Prometheus HTTP handler
	promHandler http.Handler
}

func New(da DataAccess) Agent {
	// TODO: Create drivers.New(runnerConfig)
	driver := docker.NewDocker(drivers.Config{})

	a := &agent{
		da:          da,
		driver:      driver,
		slotMgr:     NewSlotQueueMgr(),
		resources:   NewResourceTracker(),
		shutdown:    make(chan struct{}),
		promHandler: promhttp.Handler(),
	}

	// TODO assert that agent doesn't get started for API nodes up above ?
	a.wg.Add(1)
	go a.asyncDequeue() // safe shutdown can nanny this fine

	return a
}

// TODO shuffle this around somewhere else (maybe)
func (a *agent) Enqueue(ctx context.Context, call *models.Call) error {
	return a.da.Enqueue(ctx, call)
}

func (a *agent) Close() error {
	a.shutonce.Do(func() {
		close(a.shutdown)
	})

	a.wg.Wait()
	return nil
}

func (a *agent) Submit(callI Call) error {
	a.wg.Add(1)
	defer a.wg.Done()

	select {
	case <-a.shutdown:
		return models.ErrCallTimeoutServerBusy
	default:
	}

	call := callI.(*call)

	ctx, cancel := context.WithDeadline(call.req.Context(), call.execDeadline)
	call.req = call.req.WithContext(ctx)
	defer cancel()

	ctx, finish := statSpans(ctx, call)
	defer finish()

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
	a.stats.Enqueue(ctx, call.AppName, call.Path)

	a.startStateTrackers(ctx, call)
	defer a.endStateTrackers(ctx, call)

	slot, err := a.getSlot(ctx, call)
	if err != nil {
		a.handleStatsDequeue(ctx, call, err)
		return transformTimeout(err, true)
	}

	defer slot.Close() // notify our slot is free once we're done

	err = call.Start(ctx)
	if err != nil {
		a.handleStatsDequeue(ctx, call, err)
		return transformTimeout(err, true)
	}

	// decrement queued count, increment running count
	a.stats.DequeueAndStart(ctx, call.AppName, call.Path)

	// pass this error (nil or otherwise) to end directly, to store status, etc
	err = slot.exec(ctx, call)
	a.handleStatsEnd(ctx, call, err)

	// TODO: we need to allocate more time to store the call + logs in case the call timed out,
	// but this could put us over the timeout if the call did not reply yet (need better policy).
	ctx = opentracing.ContextWithSpan(context.Background(), opentracing.SpanFromContext(ctx))
	err = call.End(ctx, err)
	return transformTimeout(err, false)
}

func transformTimeout(e error, isRetriable bool) error {
	if e == context.DeadlineExceeded {
		if isRetriable {
			return models.ErrCallTimeoutServerBusy
		}
		return models.ErrCallTimeout
	}
	return e
}

// handleStatsDequeue handles stats for dequeuing for early exit (getSlot or Start)
// cases. Only timeouts can be a simple dequeue while other cases are actual errors.
func (a *agent) handleStatsDequeue(ctx context.Context, call *call, err error) {
	if err == context.DeadlineExceeded {
		a.stats.Dequeue(ctx, call.AppName, call.Path)
		IncrementTooBusy(ctx)
	} else {
		a.stats.DequeueAndFail(ctx, call.AppName, call.Path)
		IncrementErrors(ctx)
	}
}

// handleStatsEnd handles stats for after a call is ran, depending on error.
func (a *agent) handleStatsEnd(ctx context.Context, call *call, err error) {
	if err == nil {
		// decrement running count, increment completed count
		a.stats.Complete(ctx, call.AppName, call.Path)
	} else {
		// decrement running count, increment failed count
		a.stats.Failed(ctx, call.AppName, call.Path)
		// increment the timeout or errors count, as appropriate
		if err == context.DeadlineExceeded {
			IncrementTimedout(ctx)
		} else {
			IncrementErrors(ctx)
		}
	}
}

func statSpans(ctx context.Context, call *call) (_ context.Context, finish func()) {
	// agent_submit_global has no parent span because we don't want it to inherit fn_appname or fn_path
	spanGlobal := opentracing.StartSpan("agent_submit_global")

	// agent_submit_global has no parent span because we don't want it to inherit fn_path
	spanApp := opentracing.StartSpan("agent_submit_app")
	spanApp.SetBaggageItem("fn_appname", call.AppName)

	// agent_submit has a parent span in the usual way
	// it doesn't matter if it inherits fn_appname or fn_path (and we set them here in any case)
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_submit")
	span.SetBaggageItem("fn_appname", call.AppName)
	span.SetBaggageItem("fn_path", call.Path)

	return ctx, func() {
		spanGlobal.Finish()
		spanApp.Finish()
		span.Finish()
	}
}

// getSlot returns a Slot (or error) for the request to run. Depending on hot/cold
// request type, this may launch a new container or wait for other containers to become idle
// or it may wait for resources to become available to launch a new container.
func (a *agent) getSlot(ctx context.Context, call *call) (Slot, error) {
	// start the deadline context for waiting for slots
	ctx, cancel := context.WithDeadline(ctx, call.slotDeadline)
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_get_slot")
	defer span.Finish()

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
	timeout := time.Duration(60) * time.Minute
	idleTimeout := time.Duration(call.IdleTimeout) * time.Second * 2
	if timeout < idleTimeout {
		timeout = idleTimeout
	}

	logger := common.Logger(ctx)
	logger.WithField("launcher_timeout", timeout).Info("Hot function launcher starting")

	// IMPORTANT: get a context that has a child span / logger but NO timeout
	// TODO this is a 'FollowsFrom'
	ctx = opentracing.ContextWithSpan(common.WithLogger(context.Background(), logger), opentracing.SpanFromContext(ctx))
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_hot_launcher")
	defer span.Finish()

	for {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		a.checkLaunch(ctx, call)

		select {
		case <-a.shutdown: // server shutdown
			cancel()
			return
		case <-ctx.Done(): // timed out
			cancel()
			if a.slotMgr.deleteSlotQueue(call.slots) {
				logger.Info("Hot function launcher timed out")
				return
			}
		case <-call.slots.signaller:
			cancel()
		}
	}
}

func (a *agent) checkLaunch(ctx context.Context, call *call) {
	curStats := call.slots.getStats()
	isAsync := call.Type == models.TypeAsync
	isNeeded := isNewContainerNeeded(&curStats)
	common.Logger(ctx).WithFields(logrus.Fields{"currentStats": curStats, "isNeeded": isNeeded}).Debug("Hot function launcher stats")
	if !isNeeded {
		return
	}

	state := NewContainerState()
	state.UpdateState(ctx, ContainerStateWait, call.slots)

	common.Logger(ctx).WithFields(logrus.Fields{"currentStats": call.slots.getStats(), "isNeeded": isNeeded}).Info("Hot function launcher starting hot container")

	select {
	case tok := <-a.resources.GetResourceToken(ctx, call.Memory, uint64(call.CPUs), isAsync):
		a.wg.Add(1) // add waiter in this thread
		go func() {
			// NOTE: runHot will not inherit the timeout from ctx (ignore timings)
			a.runHot(ctx, call, tok, state)
			a.wg.Done()
		}()
	case <-ctx.Done(): // timeout
		state.UpdateState(ctx, ContainerStateDone, call.slots)
	case <-a.shutdown: // server shutdown
		state.UpdateState(ctx, ContainerStateDone, call.slots)
	}
}

// waitHot pings and waits for a hot container from the slot queue
func (a *agent) waitHot(ctx context.Context, call *call) (Slot, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_wait_hot")
	defer span.Finish()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // shut down dequeuer if we grab a slot

	ch := call.slots.startDequeuer(ctx)

	// 1) if we can get a slot immediately, grab it.
	// 2) if we don't, send a signaller every 200ms until we do.

	sleep := 1 * time.Microsecond // pad, so time.After doesn't send immediately
	for {
		select {
		case s := <-ch:
			if s.acquireSlot() {
				if s.slot.Error() != nil {
					s.slot.Close()
					return nil, s.slot.Error()
				}
				return s.slot, nil
			}
			// we failed to take ownership of the token (eg. container idle timeout) => try again
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-a.shutdown: // server shutdown
			return nil, models.ErrCallTimeoutServerBusy
		case <-time.After(sleep):
			// ping dequeuer again
		}

		// set sleep to 200ms after first iteration
		sleep = 200 * time.Millisecond
		// send a notification to launchHot()
		select {
		case call.slots.signaller <- true:
		default:
		}
	}
}

// launchCold waits for necessary resources to launch a new container, then
// returns the slot for that new container to run the request on.
func (a *agent) launchCold(ctx context.Context, call *call) (Slot, error) {
	isAsync := call.Type == models.TypeAsync
	ch := make(chan Slot)

	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_launch_cold")
	defer span.Finish()

	call.containerState.UpdateState(ctx, ContainerStateWait, call.slots)

	select {
	case tok := <-a.resources.GetResourceToken(ctx, call.Memory, uint64(call.CPUs), isAsync):
		go a.prepCold(ctx, call, tok, ch)
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// wait for launch err or a slot to open up
	select {
	case s := <-ch:
		if s.Error() != nil {
			s.Close()
			return nil, s.Error()
		}
		return s, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// implements Slot
type coldSlot struct {
	cookie drivers.Cookie
	tok    ResourceToken
	err    error
}

func (s *coldSlot) Error() error {
	return s.err
}

func (s *coldSlot) exec(ctx context.Context, call *call) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_cold_exec")
	defer span.Finish()

	call.requestState.UpdateState(ctx, RequestStateExec, call.slots)
	call.containerState.UpdateState(ctx, ContainerStateBusy, call.slots)

	waiter, err := s.cookie.Run(ctx)
	if err != nil {
		return err
	}

	res, err := waiter.Wait(ctx)
	if err != nil {
		return err
	} else if res.Error() != nil {
		// check for call error (oom/exit) and beam it up
		return res.Error()
	}

	// nil or timed out
	return ctx.Err()
}

func (s *coldSlot) Close() error {
	if s.cookie != nil {
		// call this from here so that in exec we don't have to eat container
		// removal latency
		s.cookie.Close(context.Background()) // ensure container removal, separate ctx
	}
	if s.tok != nil {
		s.tok.Close()
	}
	return nil
}

// implements Slot
type hotSlot struct {
	done      chan<- struct{} // signal we are done with slot
	errC      <-chan error    // container error
	container *container      // TODO mask this
	err       error
}

func (s *hotSlot) Close() error {
	close(s.done)
	return nil
}

func (s *hotSlot) Error() error {
	return s.err
}

func (s *hotSlot) exec(ctx context.Context, call *call) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_hot_exec")
	defer span.Finish()

	call.requestState.UpdateState(ctx, RequestStateExec, call.slots)

	// link the container id and id in the logs [for us!]
	common.Logger(ctx).WithField("container_id", s.container.id).Info("starting call")

	// swap in fresh pipes & stat accumulator to not interlace with other calls that used this slot [and timed out]
	stdinRead, stdinWrite := io.Pipe()
	stdoutRead, stdoutWrite := io.Pipe()
	defer stdinRead.Close()
	defer stdoutWrite.Close()

	proto := protocol.New(protocol.Protocol(call.Format), stdinWrite, stdoutRead)

	swapBack := s.container.swap(stdinRead, stdoutWrite, call.stderr, &call.Stats)
	defer swapBack() // NOTE: it's important this runs before the pipes are closed.

	errApp := make(chan error, 1)
	go func() {
		ci := protocol.NewCallInfo(call.Call, call.req)
		errApp <- proto.Dispatch(ctx, ci, call.w)
	}()

	select {
	case err := <-s.errC: // error from container
		return err
	case err := <-errApp: // from dispatch
		return err
	case <-ctx.Done(): // call timeout
		return ctx.Err()
	}
}

func (a *agent) prepCold(ctx context.Context, call *call, tok ResourceToken, ch chan Slot) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_prep_cold")
	defer span.Finish()

	call.containerState.UpdateState(ctx, ContainerStateStart, call.slots)

	// add Fn-specific information to the config to shove everything into env vars for cold
	call.Config["FN_DEADLINE"] = strfmt.DateTime(call.execDeadline).String()
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
		timeout: time.Duration(call.Timeout) * time.Second, // this is unnecessary, but in case removal fails...
		stdin:   call.req.Body,
		stdout:  call.w,
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
		slot.Close()
	}
}

func (a *agent) runHot(ctx context.Context, call *call, tok ResourceToken, state ContainerState) {
	// IMPORTANT: get a context that has a child span / logger but NO timeout
	// TODO this is a 'FollowsFrom'
	ctx = opentracing.ContextWithSpan(context.Background(), opentracing.SpanFromContext(ctx))
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_run_hot")
	defer span.Finish()
	defer tok.Close() // IMPORTANT: this MUST get called

	state.UpdateState(ctx, ContainerStateStart, call.slots)
	defer state.UpdateState(ctx, ContainerStateDone, call.slots)

	cid := id.New().String()

	// set up the stderr to capture any logs before the slot is executed and
	// between hot functions
	stderr := newLineWriter(&logWriter{
		logrus.WithFields(logrus.Fields{"between_log": true, "app_name": call.AppName, "path": call.Path, "image": call.Image, "container_id": cid}),
	})

	// between calls we need a reader that doesn't do anything
	stdin := &ghostReader{cond: sync.NewCond(new(sync.Mutex)), inner: new(waitReader)}
	defer stdin.Close()

	container := &container{
		id:     cid, // XXX we could just let docker generate ids...
		image:  call.Image,
		env:    map[string]string(call.Config),
		memory: call.Memory,
		cpus:   uint64(call.CPUs),
		stdin:  stdin,
		stdout: &ghostWriter{inner: stderr},
		stderr: &ghostWriter{inner: stderr},
	}

	logger := logrus.WithFields(logrus.Fields{"id": container.id, "app": call.AppName, "route": call.Path, "image": call.Image, "memory": call.Memory, "cpus": call.CPUs, "format": call.Format, "idle_timeout": call.IdleTimeout})
	ctx = common.WithLogger(ctx, logger)

	cookie, err := a.driver.Prepare(ctx, container)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), err: err})
		return
	}
	defer cookie.Close(context.Background()) // ensure container removal, separate ctx

	waiter, err := cookie.Run(ctx)
	if err != nil {
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), err: err})
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
			case <-a.shutdown: // server shutdown
				return
			default: // ok
			}

			done := make(chan struct{})
			state.UpdateState(ctx, ContainerStateIdle, call.slots)
			s := call.slots.queueSlot(&hotSlot{done, errC, container, nil})

			select {
			case <-s.trigger:
			case <-time.After(time.Duration(call.IdleTimeout) * time.Second):
				if call.slots.ejectSlot(s) {
					logger.Info("Canceling inactive hot function")
					return
				}
			case <-ctx.Done(): // container shutdown
				if call.slots.ejectSlot(s) {
					return
				}
			case <-a.shutdown: // server shutdown
				if call.slots.ejectSlot(s) {
					return
				}
			}

			state.UpdateState(ctx, ContainerStateBusy, call.slots)
			// IMPORTANT: if we fail to eject the slot, it means that a consumer
			// just dequeued this and acquired the slot. In other words, we were
			// late in ejectSlots(), so we have to execute this request in this
			// iteration. Beginning of for-loop will re-check ctx/shutdown case
			// and terminate after this request is done.

			// wait for this call to finish
			// NOTE do NOT select with shutdown / other channels. slot handles this.
			<-done
		}
	}()

	res, err := waiter.Wait(ctx)
	if err != nil {
		errC <- err
	} else if res.Error() != nil {
		err = res.Error()
		errC <- err
	}

	logger.WithError(err).Info("hot function terminated")
}

// container implements drivers.ContainerTask container is the execution of a
// single container, which may run multiple functions [consecutively]. the id
// and stderr can be swapped out by new calls in the container.  input and
// output must be copied in and out.
type container struct {
	id      string // contrived
	image   string
	env     map[string]string
	memory  uint64
	cpus    uint64
	timeout time.Duration // cold only (superfluous, but in case)

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	// lock protects the stats swapping
	statsMu sync.Mutex
	stats   *drivers.Stats
}

func (c *container) swap(stdin io.Reader, stdout, stderr io.Writer, cs *drivers.Stats) func() {
	ostdin := c.stdin.(*ghostReader).inner
	ostdout := c.stdout.(*ghostWriter).inner
	ostderr := c.stderr.(*ghostWriter).inner

	// if tests don't catch this, then fuck me
	c.stdin.(*ghostReader).swap(stdin)
	c.stdout.(*ghostWriter).swap(stdout)
	c.stderr.(*ghostWriter).swap(stderr)

	c.statsMu.Lock()
	ocs := c.stats
	c.stats = cs
	c.statsMu.Unlock()

	return func() {
		c.stdin.(*ghostReader).swap(ostdin)
		c.stdout.(*ghostWriter).swap(ostdout)
		c.stderr.(*ghostWriter).swap(ostderr)
		c.statsMu.Lock()
		c.stats = ocs
		c.statsMu.Unlock()
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

// WriteStat publishes each metric in the specified Stats structure as a histogram metric
func (c *container) WriteStat(ctx context.Context, stat drivers.Stat) {

	// Convert each metric value from uint64 to float64
	// and, for backward compatibility reasons, prepend each metric name with "docker_stats_fn_"
	// (if we don't care about compatibility then we can remove that)
	var metrics = make(map[string]float64)
	for key, value := range stat.Metrics {
		metrics["docker_stats_fn_"+key] = float64(value)
	}

	common.PublishHistograms(ctx, metrics)

	c.statsMu.Lock()
	if c.stats != nil {
		*(c.stats) = append(*(c.stats), stat)
	}
	c.statsMu.Unlock()
}

//func (c *container) DockerAuth() (docker.AuthConfiguration, error) {
// Implementing the docker.AuthConfiguration interface.
// TODO per call could implement this stored somewhere (vs. configured on host)
//}

// ghostWriter is an io.Writer who will pass writes to an inner writer
// that may be changed at will. it is thread safe to swap or write.
type ghostWriter struct {
	sync.Mutex
	inner io.Writer
}

func (g *ghostWriter) swap(w io.Writer) (old io.Writer) {
	g.Lock()
	old = g.inner
	g.inner = w
	g.Unlock()
	return old
}

func (g *ghostWriter) Write(b []byte) (int, error) {
	// we don't need to serialize writes but swapping g.inner could be a race if unprotected
	g.Lock()
	w := g.inner
	g.Unlock()
	n, err := w.Write(b)
	if err == io.ErrClosedPipe {
		// NOTE: we need to mask this error so that docker does not get an error
		// from writing the output stream and shut down the container.
		err = nil
	}
	return n, err
}

// ghostReader is an io.ReadCloser who will pass reads to an inner reader
// that may be changed at will. it is thread safe to swap or read.
// Read will wait for a 'real' reader if inner is of type *waitReader.
// Close must be called to prevent any pending readers from leaking.
type ghostReader struct {
	cond   *sync.Cond
	inner  io.Reader
	closed bool
}

func (g *ghostReader) swap(r io.Reader) {
	g.cond.L.Lock()
	g.inner = r
	g.cond.L.Unlock()
	g.cond.Broadcast()
}

func (g *ghostReader) Close() {
	g.cond.L.Lock()
	g.closed = true
	g.cond.L.Unlock()
	g.cond.Broadcast()
}

func (g *ghostReader) awaitRealReader() (io.Reader, bool) {
	// wait for a real reader
	g.cond.L.Lock()
	for {
		if g.closed { // check this first
			g.cond.L.Unlock()
			return nil, false
		}
		if _, ok := g.inner.(*waitReader); ok {
			g.cond.Wait()
		} else {
			break
		}
	}

	// we don't need to serialize reads but swapping g.inner could be a race if unprotected
	r := g.inner
	g.cond.L.Unlock()
	return r, true
}

func (g *ghostReader) Read(b []byte) (int, error) {
	r, ok := g.awaitRealReader()
	if !ok {
		return 0, io.EOF
	}

	n, err := r.Read(b)
	if err == io.ErrClosedPipe {
		// NOTE: we need to mask this error so that docker does not get an error
		// from reading the input stream and shut down the container.
		err = nil
	}
	return n, err
}

// waitReader returns io.EOF if anyone calls Read. don't call Read, this is a sentinel type
type waitReader struct{}

func (e *waitReader) Read([]byte) (int, error) {
	panic("read on waitReader should not happen")
}
