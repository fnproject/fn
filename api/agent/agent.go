package agent

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/agent/protocol"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// TODO we should prob store async calls in db immediately since we're returning id (will 404 until post-execution)
// TODO async calls need to add route.Headers as well
// TODO need to shut off reads/writes in dispatch to the pipes when call times out so that
// 2 calls don't have the same container's pipes...
// TODO add spans back around container launching for hot (follows from?) + other more granular spans
// TODO handle timeouts / no response in sync & async (sync is json+503 atm, not 504, async is empty log+status)
// see also: server/runner.go wrapping the response writer there, but need to handle async too (push down?)
// TODO storing logs / call can push call over the timeout
// TODO discuss concrete policy for hot launch or timeout / timeout vs time left
// TODO if we don't cap the number of any one container we could get into a situation
// where the machine is full but all the containers are idle up to the idle timeout. meh.
// TODO async is still broken, but way less so. we need to modify mq semantics
// to be much more robust. now we're at least running it if we delete the msg,
// but we may never store info about that execution so still broked (if fn
// dies). need coordination w/ db.
// TODO if a cold call times out but container is created but hasn't replied, could
// end up that the client doesn't get a reply until long after the timeout (b/c of container removal, async it?)
// TODO the log api should be plaintext (or at least offer it)
// TODO between calls, logs and stderr can contain output/ids from previous call. need elegant solution. grossness.
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
func (a *agent) handleStatsDequeue(err error, callI Call) {
	if err == context.DeadlineExceeded {
		a.stats.Dequeue(callI.Model().AppName, callI.Model().Path)
	} else {
		a.stats.DequeueAndFail(callI.Model().AppName, callI.Model().Path)
	}
}

func (a *agent) Submit(callI Call) error {
	a.wg.Add(1)
	defer a.wg.Done()

	select {
	case <-a.shutdown:
		return errors.New("agent shut down")
	default:
	}

	// increment queued count
	a.stats.Enqueue(callI.Model().AppName, callI.Model().Path)

	call := callI.(*call)
	ctx := call.req.Context()

	// agent_submit_global has no parent span because we don't want it to inherit fn_appname or fn_path
	span_global := opentracing.StartSpan("agent_submit_global")
	defer span_global.Finish()

	// agent_submit_global has no parent span because we don't want it to inherit fn_path
	span_app := opentracing.StartSpan("agent_submit_app")
	span_app.SetBaggageItem("fn_appname", callI.Model().AppName)
	defer span_app.Finish()

	// agent_submit has a parent span in the usual way
	// it doesn't matter if it inherits fn_appname or fn_path (and we set them here in any case)
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_submit")
	span.SetBaggageItem("fn_appname", callI.Model().AppName)
	span.SetBaggageItem("fn_path", callI.Model().Path)
	defer span.Finish()

	// start the timer STAT! TODO add some wiggle room
	ctx, cancel := context.WithTimeout(ctx, time.Duration(call.Timeout)*time.Second)
	call.req = call.req.WithContext(ctx)
	defer cancel()

	slot, err := a.getSlot(ctx, call) // find ram available / running
	if err != nil {
		a.handleStatsDequeue(err, call)
		return transformTimeout(err, true)
	}
	// TODO if the call times out & container is created, we need
	// to make this remove the container asynchronously?
	defer slot.Close() // notify our slot is free once we're done

	// TODO Start is checking the timer now, we could do it here, too.
	err = call.Start(ctx)
	if err != nil {
		a.handleStatsDequeue(err, call)
		return transformTimeout(err, true)
	}

	// decrement queued count, increment running count
	a.stats.DequeueAndStart(callI.Model().AppName, callI.Model().Path)

	err = slot.exec(ctx, call)
	// pass this error (nil or otherwise) to end directly, to store status, etc
	// End may rewrite the error or elect to return it

	if err == nil {
		// decrement running count, increment completed count
		a.stats.Complete(callI.Model().AppName, callI.Model().Path)
	} else {
		// decrement running count, increment failed count
		a.stats.Failed(callI.Model().AppName, callI.Model().Path)
	}

	// TODO: we need to allocate more time to store the call + logs in case the call timed out,
	// but this could put us over the timeout if the call did not reply yet (need better policy).
	ctx = opentracing.ContextWithSpan(context.Background(), span)
	err = call.End(ctx, err)
	return transformTimeout(err, false)
}

// getSlot returns a Slot (or error) for the request to run. Depending on hot/cold
// request type, this may launch a new container or wait for other containers to become idle
// or it may wait for resources to become available to launch a new container.
func (a *agent) getSlot(ctx context.Context, call *call) (Slot, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_get_slot")
	defer span.Finish()

	isHot := protocol.IsStreamable(protocol.Protocol(call.Format))
	if isHot {
		// For hot requests, we use a long lived slot queue, which we use to manage hot containers
		call.slots = a.slotMgr.getHotSlotQueue(call)
		start := time.Now()

		call.slots.enterState(SlotQueueWaiter)
		s, err := a.launchHot(ctx, call)
		call.slots.exitStateWithLatency(SlotQueueWaiter, uint64(time.Now().Sub(start).Seconds()*1000))

		return s, err
	}

	return a.launchCold(ctx, call)
}

// launchHot checks with slot queue to see if a new container needs to be launched and waits
// for available slots in the queue for hot request execution.
func (a *agent) launchHot(ctx context.Context, call *call) (Slot, error) {

	isAsync := call.Type == models.TypeAsync

launchLoop:
	for {
		// Check/evaluate if we need to launch a new hot container
		doLaunch, stats := call.slots.isNewContainerNeeded()
		common.Logger(ctx).WithField("stats", stats).Debug("checking hot container launch ", doLaunch)

		if doLaunch {
			ctxToken, tokenCancel := context.WithCancel(context.Background())

			// wait on token/slot/timeout whichever comes first
			select {
			case tok, isOpen := <-a.resources.GetResourceToken(ctxToken, call.Memory, isAsync):
				tokenCancel()
				if !isOpen {
					return nil, models.ErrCallTimeoutServerBusy
				}

				a.wg.Add(1)
				go a.runHot(ctx, call, tok)
			case s, ok := <-call.slots.getDequeueChan():
				tokenCancel()
				if !ok {
					return nil, errors.New("slot shut down while waiting for hot slot")
				}
				if s.acquireSlot() {
					if s.slot.Error() != nil {
						s.slot.Close()
						return nil, s.slot.Error()
					}
					return s.slot, nil
				}

				// we failed to take ownership of the token (eg. container idle timeout)
				// try launching again
				continue launchLoop
			case <-ctx.Done():
				tokenCancel()
				return nil, ctx.Err()
			}
		}

		// After launching (if it was necessary) a container, now wait for slot/timeout
		// or periodically reevaluate the launchHot() logic from beginning.
		select {
		case s, ok := <-call.slots.getDequeueChan():
			if !ok {
				return nil, errors.New("slot shut down while waiting for hot slot")
			}
			if s.acquireSlot() {
				if s.slot.Error() != nil {
					s.slot.Close()
					return nil, s.slot.Error()
				}
				return s.slot, nil
			}

			// we failed to take ownership of the token (eg. container idle timeout)
			// try launching again
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(200) * time.Millisecond):
			// reevaluate
		}
	}
}

// launchCold waits for necessary resources to launch a new container, then
// returns the slot for that new container to run the request on.
func (a *agent) launchCold(ctx context.Context, call *call) (Slot, error) {

	isAsync := call.Type == models.TypeAsync
	ch := make(chan Slot)

	select {
	case tok, isOpen := <-a.resources.GetResourceToken(ctx, call.Memory, isAsync):
		if !isOpen {
			return nil, models.ErrCallTimeoutServerBusy
		}
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
	proto     protocol.ContainerIO
	errC      <-chan error // container error
	container *container   // TODO mask this
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

	// link the container id and id in the logs [for us!]
	common.Logger(ctx).WithField("container_id", s.container.id).Info("starting call")

	start := time.Now()
	defer func() {
		call.slots.recordLatency(SlotQueueRunner, uint64(time.Now().Sub(start).Seconds()*1000))
	}()

	// swap in the new stderr logger & stat accumulator
	oldStderr := s.container.swap(call.stderr, &call.Stats)
	defer s.container.swap(oldStderr, nil) // once we're done, swap out in this scope to prevent races

	errApp := make(chan error, 1)
	go func() {
		// TODO make sure stdin / stdout not blocked if container dies or we leak goroutine
		// we have to make sure this gets shut down or 2 threads will be reading/writing in/out
		ci := protocol.NewCallInfo(call.Model(), call.req)
		errApp <- s.proto.Dispatch(ctx, ci, call.w)
	}()

	select {
	case err := <-s.errC: // error from container
		return err
	case err := <-errApp: // from dispatch
		return err
	case <-ctx.Done(): // call timeout
		return ctx.Err()
	}

	// TODO we REALLY need to wait for dispatch to return before conceding our slot
}

func (a *agent) prepCold(ctx context.Context, call *call, tok ResourceToken, ch chan Slot) {
	container := &container{
		id:      id.New().String(), // XXX we could just let docker generate ids...
		image:   call.Image,
		env:     call.EnvVars, // full env
		memory:  call.Memory,
		timeout: time.Duration(call.Timeout) * time.Second, // this is unnecessary, but in case removal fails...
		stdin:   call.req.Body,
		stdout:  call.w,
		stderr:  call.stderr,
		stats:   &call.Stats,
	}

	// pull & create container before we return a slot, so as to be friendly
	// about timing out if this takes a while...
	cookie, err := a.driver.Prepare(ctx, container)
	slot := &coldSlot{cookie, tok, err}
	select {
	case ch <- slot:
	case <-ctx.Done():
		slot.Close()
	}
}

func (a *agent) runHot(ctxArg context.Context, call *call, tok ResourceToken) {
	// We must be careful to only use ctxArg for logs/spans
	defer a.wg.Done()

	// create a span from ctxArg but ignore the new Context
	// instead we will create a new Context below and explicitly set its span
	span, _ := opentracing.StartSpanFromContext(ctxArg, "docker_run_hot")
	defer span.Finish()
	defer tok.Close()

	// TODO we have to make sure we flush these pipes or we will deadlock
	stdinRead, stdinWrite := io.Pipe()
	stdoutRead, stdoutWrite := io.Pipe()

	proto := protocol.New(protocol.Protocol(call.Format), stdinWrite, stdoutRead)

	// we don't want to timeout in here. this is inside of a goroutine and the
	// caller can timeout this Call appropriately. e.g. w/ hot if it takes 20
	// minutes to pull, then timing out calls for 20 minutes and eventually
	// having the image is ideal vs. never getting the image pulled.
	// TODO this ctx needs to inherit logger, etc
	ctx, shutdownContainer := context.WithCancel(context.Background())
	defer shutdownContainer() // close this if our waiter returns

	// add the span we created above to the new Context
	ctx = opentracing.ContextWithSpan(ctx, span)

	start := time.Now()
	call.slots.enterState(SlotQueueStarter)

	cid := id.New().String()

	// set up the stderr for the first one to capture any logs before the slot is
	// executed and between hot functions TODO this is still a little tobias funke
	stderr := newLineWriter(&logWriter{
		logrus.WithFields(logrus.Fields{"between_log": true, "app_name": call.AppName, "path": call.Path, "image": call.Image, "container_id": cid}),
	})

	container := &container{
		id:     cid, // XXX we could just let docker generate ids...
		image:  call.Image,
		env:    call.BaseEnv, // only base env
		memory: call.Memory,
		stdin:  stdinRead,
		stdout: stdoutWrite,
		stderr: &ghostWriter{inner: stderr},
	}

	logger := logrus.WithFields(logrus.Fields{"id": container.id, "app": call.AppName, "route": call.Path, "image": call.Image, "memory": call.Memory, "format": call.Format, "idle_timeout": call.IdleTimeout})
	ctx = common.WithLogger(ctx, logger)

	cookie, err := a.driver.Prepare(ctx, container)
	if err != nil {
		call.slots.exitStateWithLatency(SlotQueueStarter, uint64(time.Now().Sub(start).Seconds()*1000))
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), err: err})
		return
	}
	defer cookie.Close(context.Background()) // ensure container removal, separate ctx

	waiter, err := cookie.Run(ctx)
	if err != nil {
		call.slots.exitStateWithLatency(SlotQueueStarter, uint64(time.Now().Sub(start).Seconds()*1000))
		call.slots.queueSlot(&hotSlot{done: make(chan struct{}), err: err})
		return
	}

	// container is running
	call.slots.enterState(SlotQueueRunner)
	call.slots.exitStateWithLatency(SlotQueueStarter, uint64(time.Now().Sub(start).Seconds()*1000))
	defer call.slots.exitState(SlotQueueRunner)

	// buffered, in case someone has slot when waiter returns but isn't yet listening
	errC := make(chan error, 1)

	go func() {
		for {
			select { // make sure everything is up before trying to send slot
			case <-ctx.Done(): // container shutdown
				return
			case <-a.shutdown: // server shutdown
				shutdownContainer()
				return
			default: // ok
			}

			done := make(chan struct{})
			s := call.slots.queueSlot(&hotSlot{done, proto, errC, container, nil})

			select {
			case <-s.trigger:
			case <-time.After(time.Duration(call.IdleTimeout) * time.Second):
				if call.slots.ejectSlot(s) {
					logger.Info("Canceling inactive hot function")
					shutdownContainer()
					return
				}
			case <-ctx.Done(): // container shutdown
				if call.slots.ejectSlot(s) {
					return
				}
			case <-a.shutdown: // server shutdown
				if call.slots.ejectSlot(s) {
					shutdownContainer()
					return
				}
			}
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
		errC <- res.Error()
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
	timeout time.Duration // cold only (superfluous, but in case)

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	// lock protects the swap and any fields that need to be swapped
	sync.Mutex
	stats *drivers.Stats
}

func (c *container) swap(stderr io.Writer, cs *drivers.Stats) (old io.Writer) {
	c.Lock()
	defer c.Unlock()

	// TODO meh, maybe shouldn't bury this
	old = c.stderr
	gw, ok := c.stderr.(*ghostWriter)
	if ok {
		old = gw.swap(stderr)
	}

	c.stats = cs
	return old
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

// Log the specified stats to a tracing span.
// Spans are not processed by the collector until the span ends, so to prevent any delay
// in processing the stats when the function is long-lived we create a new span for every call
func (c *container) WriteStat(ctx context.Context, stat drivers.Stat) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "docker_stats")
	defer span.Finish()
	for key, value := range stat.Metrics {
		span.LogFields(log.Uint64("fn_"+key, value))
	}

	c.Lock()
	defer c.Unlock()
	if c.stats != nil {
		*(c.stats) = append(*(c.stats), stat)
	}
}

//func (c *container) DockerAuth() (docker.AuthConfiguration, error) {
// Implementing the docker.AuthConfiguration interface.
// TODO per call could implement this stored somewhere (vs. configured on host)
//}

// ghostWriter is a writer who will pass writes to an inner writer
// (that may be changed at will).
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
	return w.Write(b)
}
