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
	"github.com/opentracing/opentracing-go"
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
// TODO if we don't cap the number of any one container we could get into a situation
// where the machine is full but all the containers are idle up to the idle timeout. meh.
// TODO async is still broken, but way less so. we need to modify mq semantics
// to be much more robust. now we're at least running it if we delete the msg,
// but we may never store info about that execution so still broked (if fn
// dies). need coordination w/ db.
// TODO if a cold call times out but container is created but hasn't replied, could
// end up that the client doesn't get a reply until long after the timeout (b/c of container removal, async it?)
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

func (a *agent) submit(ctx context.Context, call *call) error {
	a.stats.Enqueue(ctx, call.AppName, call.Path)

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
		// note that this is not a timeout from the perspective of the caller, so don't increment the timeout count
	} else {
		a.stats.DequeueAndFail(ctx, call.AppName, call.Path)
		a.stats.IncrementErrors(ctx)
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
			a.stats.IncrementTimedout(ctx)
		} else {
			a.stats.IncrementErrors(ctx)
		}
	}
}

func statSpans(ctx context.Context, call *call) (ctxr context.Context, finish func()) {
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

	isHot := protocol.IsStreamable(protocol.Protocol(call.Format))
	if isHot {
		start := time.Now()

		// For hot requests, we use a long lived slot queue, which we use to manage hot containers
		var isNew bool
		call.slots, isNew = a.slotMgr.getSlotQueue(call)
		if isNew {
			go a.hotLauncher(ctx, call)
		}

		s, err := a.waitHot(ctx, call)
		call.slots.exitStateWithLatency(SlotQueueWaiter, uint64(time.Now().Sub(start).Seconds()*1000))
		return s, err
	}

	return a.launchCold(ctx, call)
}

// hotLauncher is spawned in a go routine for each slot queue to monitor stats and launch hot
// containers if needed. Upon shutdown or activity timeout, hotLauncher exits and during exit,
// it destroys the slot queue.
func (a *agent) hotLauncher(ctx context.Context, callObj *call) {

	// Let use 60 minutes or 2 * IdleTimeout as hot queue idle timeout, pick
	// whichever is longer. If in this time, there's no activity, then
	// we destroy the hot queue.
	timeout := time.Duration(60) * time.Minute
	idleTimeout := time.Duration(callObj.IdleTimeout) * time.Second * 2
	if timeout < idleTimeout {
		timeout = idleTimeout
	}

	logger := common.Logger(ctx)
	logger.WithField("launcher_timeout", timeout).Info("Hot function launcher starting")
	isAsync := callObj.Type == models.TypeAsync
	prevStats := callObj.slots.getStats()

	for {
		select {
		case <-a.shutdown: // server shutdown
			return
		case <-time.After(timeout):
			if a.slotMgr.deleteSlotQueue(callObj.slots) {
				logger.Info("Hot function launcher timed out")
				return
			}
		case <-callObj.slots.signaller:
		}

		curStats := callObj.slots.getStats()
		isNeeded := isNewContainerNeeded(&curStats, &prevStats)
		prevStats = curStats
		logger.WithFields(logrus.Fields{
			"currentStats":  curStats,
			"previousStats": curStats,
		}).Debug("Hot function launcher stats")
		if !isNeeded {
			continue
		}

		resourceCtx, cancel := context.WithCancel(context.Background())
		logger.WithFields(logrus.Fields{
			"currentStats":  curStats,
			"previousStats": curStats,
		}).Info("Hot function launcher starting hot container")

		select {
		case tok, isOpen := <-a.resources.GetResourceToken(resourceCtx, callObj.Memory, uint64(callObj.CPUs), isAsync):
			cancel()
			if isOpen {
				a.wg.Add(1)
				go func(ctx context.Context, call *call, tok ResourceToken) {
					a.runHot(ctx, call, tok)
					a.wg.Done()
				}(ctx, callObj, tok)
			} else {
				// this means the resource was impossible to reserve (eg. memory size we can never satisfy)
				callObj.slots.queueSlot(&hotSlot{done: make(chan error, 1), err: models.ErrCallTimeoutServerBusy})
			}
		case <-time.After(timeout):
			cancel()
			if a.slotMgr.deleteSlotQueue(callObj.slots) {
				logger.Info("Hot function launcher timed out")
				return
			}
		case <-a.shutdown: // server shutdown
			cancel()
			return
		}
	}
}

// waitHot pings and waits for a hot container from the slot queue
func (a *agent) waitHot(ctx context.Context, call *call) (Slot, error) {

	ch, cancel := call.slots.startDequeuer(ctx)
	defer cancel()

	for {
		// send a notification to launcHot()
		select {
		case call.slots.signaller <- true:
		default:
		}

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
		case <-time.After(time.Duration(200) * time.Millisecond):
			// ping dequeuer again
		case <-a.shutdown: // server shutdown
			return nil, models.ErrCallTimeoutServerBusy
		}
	}
}

// launchCold waits for necessary resources to launch a new container, then
// returns the slot for that new container to run the request on.
func (a *agent) launchCold(ctx context.Context, call *call) (Slot, error) {

	isAsync := call.Type == models.TypeAsync
	ch := make(chan Slot)

	select {
	case tok, isOpen := <-a.resources.GetResourceToken(ctx, call.Memory, uint64(call.CPUs), isAsync):
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
	done      chan error // signal we are done with slot
	proto     protocol.ContainerIO
	errC      <-chan error // container error
	container *container   // TODO mask this
	err       error
}

func (s *hotSlot) Close() error {
	select {
	case s.done <- nil:
	default:
	}
	return nil
}

func (s *hotSlot) sendError(err error) {
	select {
	case s.done <- err:
	default:
	}
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

	containerError := make(chan error, 1)

	go func() {
		select {
		case err := <-s.errC: // error from container
			containerError <- err
		case <-ctx.Done(): // timeout or cancel from ctx
			containerError <- ctx.Err()
			s.sendError(ctx.Err())
		}
	}()

	ci := protocol.NewCallInfo(call.Call, call.req)
	dispatchError := s.proto.Dispatch(ci, call.ioWriter)

	select {
	case err := <-containerError:
		return err
	default:
	}

	// protocol I/O errors are non-recoverable, terminate the container
	// TODO: exclude client side errors from this since this punishes
	// container if client side writes fail. But if client side I/O
	// fails, then we must leave container side in good-shape (eg.
	// consume data in/out container pipes)
	if dispatchError != nil {
		s.sendError(dispatchError)
	}
	return dispatchError
}

func specialHeader(k string) bool {
	return k == "Fn_call_id" || k == "Fn_method" || k == "Fn_request_url"
}

func (a *agent) prepCold(ctx context.Context, call *call, tok ResourceToken, ch chan Slot) {
	// add additional headers to the config to shove everything into env vars for cold
	for k, v := range call.Headers {
		if !specialHeader(k) {
			k = "FN_HEADER_" + k
		} else {
			k = strings.ToUpper(k) // for compat, FN_CALL_ID, etc. in env for cold
		}
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
		stdout:  call.ioWriter,
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

	// create a span from ctxArg but ignore the new Context
	// instead we will create a new Context below and explicitly set its span
	span, _ := opentracing.StartSpanFromContext(ctxArg, "docker_run_hot")
	defer span.Finish()
	defer tok.Close()

	// TODO we have to make sure we flush these pipes or we will deadlock
	stdinRead, stdinWrite := io.Pipe()
	stdoutRead, stdoutWrite := io.Pipe()

	proto := protocol.New(protocol.Protocol(call.Format), stdinWrite, stdoutRead)

	defer stdinRead.Close()
	defer stdoutWrite.Close()

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
		env:    map[string]string(call.Config),
		memory: call.Memory,
		cpus:   uint64(call.CPUs),
		stdin:  stdinRead,
		stdout: stdoutWrite,
		stderr: &ghostWriter{inner: stderr},
	}

	logger := logrus.WithFields(logrus.Fields{"id": container.id, "app": call.AppName, "route": call.Path, "image": call.Image, "memory": call.Memory, "cpus": call.CPUs, "format": call.Format, "idle_timeout": call.IdleTimeout})
	ctx = common.WithLogger(ctx, logger)

	cookie, err := a.driver.Prepare(ctx, container)
	if err != nil {
		call.slots.exitStateWithLatency(SlotQueueStarter, uint64(time.Now().Sub(start).Seconds()*1000))
		call.slots.queueSlot(&hotSlot{done: make(chan error, 1), err: err})
		return
	}
	defer cookie.Close(context.Background()) // ensure container removal, separate ctx

	waiter, err := cookie.Run(ctx)
	if err != nil {
		call.slots.exitStateWithLatency(SlotQueueStarter, uint64(time.Now().Sub(start).Seconds()*1000))
		call.slots.queueSlot(&hotSlot{done: make(chan error, 1), err: err})
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

			done := make(chan error, 1)
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
			err := <-done
			if err != nil {
				shutdownContainer()
				return
			}
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
	cpus    uint64
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
