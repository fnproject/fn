package agent

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/agent/protocol"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/common/singleflight"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/opentracing/opentracing-go"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

// TODO make sure some errors that user should see (like image doesn't exist) bubble up
// TODO we should prob store async calls in db immediately since we're returning id (will 404 until post-execution)
// TODO async calls need to add route.Headers as well
// TODO need to shut off reads/writes in dispatch to the pipes when call times out so that
// 2 calls don't have the same container's pipes...
// TODO fix ram token / cold slot racy races (highly unlikely, still fix)
// TODO add spans back around container launching for hot (follows from?) + other more granular spans
// TODO handle timeouts / no response in sync & async (sync is json+503 atm, not 504, async is empty log+status)
// see also: server/runner.go wrapping the response writer there, but need to handle async too (push down?)
// TODO herd launch prevention part deux
// TODO plumb FXLB-WAIT back - can we use headers now? maybe let's use api
// TODO none of the Datastore methods actually use the ctx for timeouts :(
// TODO not adding padding if call times out to store appropriately (ctx timed out, happenstance it works now cuz of ^)
// TODO all Datastore methods need to take unit of tenancy (app or route) at least (e.g. not just call id)
// TODO limit the request body length when making calls
// TODO discuss concrete policy for hot launch or timeout / timeout vs time left
// TODO call env need to be map[string][]string to match headers behavior...
// TODO it may be nice to have an interchange type for Dispatch that can have
// all the info we need to build e.g. http req, grpc req, json, etc.  so that
// we can easily do e.g. http->grpc, grpc->http, http->json. ofc grpc<->http is
// weird for proto specifics like e.g. proto version, method, headers, et al.
// discuss.
// TODO if we don't cap the number of any one container we could get into a situation
// where the machine is full but all the containers are idle up to the idle timeout. meh.
// TODO async is still broken, but way less so. we need to modify mq semantics
// to be much more robust. now we're at least running it if we delete the msg,
// but we may never store info about that execution so still broked (if fn
// dies). need coordination w/ db.
// TODO if a cold call times out but container is created but hasn't replied, could
// end up that the client doesn't get a reply until long after the timeout (b/c of container removal, async it?)
// TODO we should prob not be logging all async output to the logs by default...
// TODO the call api should fill in all the fields
// TODO the log api should be plaintext (or at least offer it)
// TODO func logger needs to be hanged, dragged and quartered. in reverse order.
// TODO we should probably differentiate ran-but-timeout vs timeout-before-run
// TODO push down the app/route cache into Datastore that decorates
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
}

type agent struct {
	// TODO maybe these should be on GetCall? idk. was getting bloated.
	mq models.MessageQueue
	ds models.Datastore

	driver drivers.Driver

	hMu sync.RWMutex // protects hot
	hot map[string]chan slot

	// TODO could make this separate too...
	// cache for apps and routes
	cache        *cache.Cache
	singleflight singleflight.SingleFlight // singleflight assists Datastore // TODO rename

	// TODO we could make a separate struct for the memory stuff
	// cond protects access to ramUsed
	cond *sync.Cond
	// ramTotal is the total accessible memory by this process
	ramTotal uint64
	// ramUsed is ram reserved for running containers. idle hot containers
	// count against ramUsed.
	ramUsed uint64

	// used to track running calls / safe shutdown
	wg       sync.WaitGroup // TODO rename
	shutdown chan struct{}

	stats // TODO kill me
}

func New(ds models.Datastore, mq models.MessageQueue) Agent {
	// TODO: Create drivers.New(runnerConfig)
	driver := docker.NewDocker(drivers.Config{})

	a := &agent{
		ds:       ds,
		mq:       mq,
		driver:   driver,
		hot:      make(map[string]chan slot),
		cache:    cache.New(5*time.Second, 1*time.Minute),
		cond:     sync.NewCond(new(sync.Mutex)),
		ramTotal: getAvailableMemory(),
		shutdown: make(chan struct{}),
	}

	go a.asyncDequeue() // safe shutdown can nanny this fine

	return a
}

func (a *agent) Close() error {
	select {
	case <-a.shutdown:
	default:
		close(a.shutdown)
	}
	a.wg.Wait()
	return nil
}

func (a *agent) Submit(callI Call) error {
	a.wg.Add(1)
	defer a.wg.Done()

	select {
	case <-a.shutdown:
		return errors.New("agent shut down")
	default:
	}

	a.stats.Enqueue()

	call := callI.(*call)
	ctx := call.req.Context()
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_submit")
	defer span.Finish()

	// start the timer STAT! TODO add some wiggle room
	ctx, cancel := context.WithTimeout(ctx, time.Duration(call.Timeout)*time.Second)
	call.req = call.req.WithContext(ctx)
	defer cancel()

	slot, err := a.getSlot(ctx, call) // find ram available / running
	if err != nil {
		return err
	}
	// TODO if the call times out & container is created, we need
	// to make this remove the container asynchronously?
	defer slot.Close() // notify our slot is free once we're done

	// TODO Start is checking the timer now, we could do it here, too.
	err = call.Start(ctx)
	if err != nil {
		return err
	}

	a.stats.Start()

	err = slot.exec(ctx, call)
	// pass this error (nil or otherwise) to end directly, to store status, etc
	// End may rewrite the error or elect to return it

	a.stats.Complete()

	// TODO: we need to allocate more time to store the call + logs in case the call timed out,
	// but this could put us over the timeout if the call did not reply yet (need better policy).
	ctx = opentracing.ContextWithSpan(context.Background(), span)
	call.End(ctx, err)

	return err
}

// getSlot must ensure that if it receives a slot, it will be returned, otherwise
// a container will be locked up forever waiting for slot to free.
func (a *agent) getSlot(ctx context.Context, call *call) (slot, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_get_slot")
	defer span.Finish()

	if protocol.IsStreamable(protocol.Protocol(call.Format)) {
		return a.hotSlot(ctx, call)
	}

	// make new channel and launch 1 for cold
	ch := make(chan slot)
	return a.launchOrSlot(ctx, ch, call)
}

// launchOrSlot will launch a container that will send slots on the provided channel when it
// is free if no slots are available on that channel first. the returned slot may or may not
// be from the launched container. if there is an error launching a new container (if necessary),
// then that will be returned rather than a slot, if no slot is free first.
func (a *agent) launchOrSlot(ctx context.Context, slots chan slot, call *call) (slot, error) {
	var errCh <-chan error

	// check if any slot immediately without trying to get a ram token
	select {
	case s := <-slots:
		return s, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// if nothing free, wait for ram token or a slot
	select {
	case s := <-slots:
		return s, nil
	case tok := <-a.ramToken(call.Memory * 1024 * 1024): // convert MB TODO mangle
		errCh = a.launch(ctx, slots, call, tok) // TODO mangle
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// wait for launch err or a slot to open up (possibly from launch)
	select {
	case err := <-errCh:
		// if we get a launch err, try to return to user (e.g. image not found)
		return nil, err
	case slot := <-slots:
		return slot, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a *agent) hotSlot(ctx context.Context, call *call) (slot, error) {
	slots := a.slots(hotKey(call))

	// TODO if we track avg run time we could know how long to wait or
	// if we need to launch instead of waiting.

	// if we can get a slot in a reasonable amount of time, use it
	select {
	case s := <-slots:
		return s, nil
	case <-time.After(100 * time.Millisecond): // XXX(reed): precise^
		// TODO this means the first launched container if none are running eats
		// this. yes it sucks but there are a lot of other fish to fry, opening a
		// policy discussion...
	}

	// then wait for a slot or try to launch...
	return a.launchOrSlot(ctx, slots, call)
}

// TODO this should be a LIFO stack of channels, perhaps. a queue (channel)
// will always send the least recently used, not ideal.
func (a *agent) slots(key string) chan slot {
	a.hMu.RLock()
	slots, ok := a.hot[key]
	a.hMu.RUnlock()
	if !ok {
		a.hMu.Lock()
		slots, ok = a.hot[key]
		if !ok {
			slots = make(chan slot) // should not be buffered
			a.hot[key] = slots
		}
		a.hMu.Unlock()
	}
	return slots
}

func hotKey(call *call) string {
	// return a sha1 hash of a (hopefully) unique string of all the config
	// values, to make map lookups quicker [than the giant unique string]

	hash := sha1.New()
	fmt.Fprint(hash, call.AppName, "\x00")
	fmt.Fprint(hash, call.Path, "\x00")
	fmt.Fprint(hash, call.Image, "\x00")
	fmt.Fprint(hash, call.Timeout, "\x00")
	fmt.Fprint(hash, call.IdleTimeout, "\x00")
	fmt.Fprint(hash, call.Memory, "\x00")
	fmt.Fprint(hash, call.Format, "\x00")

	// we have to sort these before printing, yay. TODO do better
	keys := make([]string, 0, len(call.BaseEnv))
	for k := range call.BaseEnv {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprint(hash, k, "\x00", call.BaseEnv[k], "\x00")
	}

	var buf [sha1.Size]byte
	return string(hash.Sum(buf[:0]))
}

// TODO we could rename this more appropriately (ideas?)
type Token interface {
	// Close must be called by any thread that receives a token.
	io.Closer
}

type token struct {
	decrement func()
}

func (t *token) Close() error {
	t.decrement()
	return nil
}

// NOTE in theory goroutines spawned here could run forever (i.e. leak), if the provided value
// is unable to be satisfied. the calling thread will time out waiting for it and the value
// properly adjusted if ever satisfied, but we could trivially provide a ctx here and time
// out if the calling thread times out if we want to [just to prevent leaks].
//
// the received token should be passed directly to launch (unconditionally), launch
// will close this token (i.e. the receiver should not call Close)
func (a *agent) ramToken(memory uint64) <-chan Token {
	// TODO we could do an initial check here and return in the same thread so
	// that a calling thread could call this and have a filled channel
	// immediately so that a a select default case could be used to determine
	// whether machine is at capacity (and caller can decide whether to wait) --
	// right now this is a race if used as described.

	c := a.cond
	ch := make(chan Token)

	go func() {
		c.L.Lock()
		for (a.ramUsed + memory) > a.ramTotal {
			// TODO we could add ctx here
			c.Wait()
		}

		a.ramUsed += memory
		c.L.Unlock()

		t := &token{decrement: func() {
			c.L.Lock()
			a.ramUsed -= memory
			c.L.Unlock()
			c.Broadcast()
		}}

		select {
		// TODO fix this race. caller needs to provide channel since we could get here
		// before ramToken has returned. :( or something better, idk
		case ch <- t:
		default:
			// if we can't send b/c nobody is waiting anymore, need to decrement here
			t.Close()
		}
	}()

	return ch
}

// asyncRam will send a signal on the returned channel when at least half of
// the available RAM on this machine is free.
func (a *agent) asyncRam() chan struct{} {
	ch := make(chan struct{})

	c := a.cond
	go func() {
		c.L.Lock()
		for (a.ramTotal/2)-a.ramUsed < 0 {
			c.Wait()
		}
		c.L.Unlock()
		ch <- struct{}{}
		// TODO this could leak forever (only in shutdown, blech)
	}()

	return ch
}

type slot interface {
	exec(ctx context.Context, call *call) error
	io.Closer
}

// implements Slot
type coldSlot struct {
	cookie drivers.Cookie
	tok    Token
	stderr io.Closer
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
	} else if res.Error() != "" {
		// check for call error (oom/exit) and beam it up
		return res
	}

	// nil or timed out (Wait will silently return nil if it encounters a timeout, maybe TODO)
	return ctx.Err()
}

func (s *coldSlot) Close() error {
	if s.cookie != nil {
		// call this from here so that in exec we don't have to eat container
		// removal latency
		s.cookie.Close(context.Background()) // ensure container removal, separate ctx
	}
	s.tok.Close()
	s.stderr.Close()
	return nil
}

// implements Slot
type hotSlot struct {
	done      chan<- struct{} // signal we are done with slot
	proto     protocol.ContainerIO
	errC      <-chan error // container error
	container *container   // TODO mask this
}

func (s *hotSlot) Close() error { close(s.done); return nil }

func (s *hotSlot) exec(ctx context.Context, call *call) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "agent_hot_exec")
	defer span.Finish()

	stderr := NewFuncLogger(ctx, call.AppName, call.Path, call.Image, call.ID, call.ds)
	if call.w == nil {
		// send STDOUT to logs if no writer given (async...)
		// TODO fuck func logger, change it to not need a context and make calls
		// require providing their own stderr and writer instead of this crap. punting atm.
		call.w = stderr
	}

	// link the container id and id in the logs [for us!]
	// TODO go is broke idk why logrus.Fields doesn't work
	common.Logger(ctx).WithField("container_id", s.container.id).WithField("id", call.ID).Info("starting call")

	// swap in the new id and the new stderr logger
	s.container.swap(stderr)
	defer stderr.Close() // TODO shove in Close / elsewhere (to upload logs after exec exits)

	errApp := make(chan error, 1)
	go func() {
		// TODO make sure stdin / stdout not blocked if container dies or we leak goroutine
		// we have to make sure this gets shut down or 2 threads will be reading/writing in/out
		errApp <- s.proto.Dispatch(call.w, call.req)
	}()

	select {
	case err := <-s.errC: // error from container
		return err
	case err := <-errApp:
		return err
	case <-ctx.Done(): // call timeout
		return ctx.Err()
	}

	// TODO we REALLY need to wait for dispatch to return before conceding our slot
}

// this will work for hot & cold (woo)
// if launch encounters a non-nil error it will send it on the returned channel,
// this can be useful if an image doesn't exist, e.g.
// TODO i don't like how this came out and it's slightly racy w/ unbuffered channels since
// the enclosing thread may not be ready to receive (up to go scheduler), need to tidy up, but
// this race is unlikely (still need to fix, yes)
func (a *agent) launch(ctx context.Context, slots chan<- slot, call *call, tok Token) <-chan error {
	ch := make(chan error, 1)

	if !protocol.IsStreamable(protocol.Protocol(call.Format)) {
		// TODO no
		go func() {
			err := a.prepCold(ctx, slots, call, tok)
			if err != nil {
				ch <- err
			}
		}()
		return ch
	}

	go func() {
		err := a.runHot(slots, call, tok)
		if err != nil {
			ch <- err
		}
	}()
	return ch
}

func (a *agent) prepCold(ctx context.Context, slots chan<- slot, call *call, tok Token) error {
	// TODO dupe stderr code, reduce me
	stderr := NewFuncLogger(ctx, call.AppName, call.Path, call.Image, call.ID, call.ds)
	if call.w == nil {
		// send STDOUT to logs if no writer given (async...)
		// TODO fuck func logger, change it to not need a context and make calls
		// require providing their own stderr and writer instead of this crap. punting atm.
		call.w = stderr
	}

	container := &container{
		id:      id.New().String(), // XXX we could just let docker generate ids...
		image:   call.Image,
		env:     call.EnvVars, // full env
		memory:  call.Memory,
		timeout: time.Duration(call.Timeout) * time.Second, // this is unnecessary, but in case removal fails...
		stdin:   call.req.Body,
		stdout:  call.w,
		stderr:  stderr,
	}

	// pull & create container before we return a slot, so as to be friendly
	// about timing out if this takes a while...
	cookie, err := a.driver.Prepare(ctx, container)
	if err != nil {
		tok.Close() // TODO make this less brittle
		return err
	}

	slot := &coldSlot{cookie, tok, stderr}
	select {
	case slots <- slot: // TODO need to make sure receiver will be ready (go routine race)
	default:
		slot.Close() // if we can't send this slot, need to take care of it ourselves
	}
	return nil
}

// TODO add ctx back but be careful to only use for logs/spans
func (a *agent) runHot(slots chan<- slot, call *call, tok Token) error {
	if tok == nil {
		// TODO we should panic, probably ;)
		return errors.New("no token provided, not giving you a slot")
	}
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

	// set up the stderr for the first one to capture any logs before the slot is
	// executed.
	// TODO need to figure out stderr logging for hot functions at a high level
	stderr := &ghostWriter{inner: newLineWriter(&logWriter{ctx: ctx, appName: call.AppName, path: call.Path, image: call.Image, reqID: call.ID})}

	container := &container{
		id:     id.New().String(), // XXX we could just let docker generate ids...
		image:  call.Image,
		env:    call.BaseEnv, // only base env
		memory: call.Memory,
		stdin:  stdinRead,
		stdout: stdoutWrite,
		stderr: stderr,
	}

	logger := logrus.WithFields(logrus.Fields{"id": container.id, "app": call.AppName, "route": call.Path, "image": call.Image, "memory": call.Memory, "format": call.Format, "idle_timeout": call.IdleTimeout})

	cookie, err := a.driver.Prepare(ctx, container)
	if err != nil {
		return err
	}
	defer cookie.Close(context.Background()) // ensure container removal, separate ctx

	waiter, err := cookie.Run(ctx)
	if err != nil {
		return err
	}

	// container is running

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
			slot := &hotSlot{done, proto, errC, container}

			select {
			case slots <- slot:
			case <-time.After(time.Duration(call.IdleTimeout) * time.Second):
				logger.Info("Canceling inactive hot function")
				shutdownContainer()
				return
			case <-ctx.Done(): // container shutdown
				return
			case <-a.shutdown: // server shutdown
				shutdownContainer()
				return
			}

			// wait for this call to finish
			// NOTE do NOT select with shutdown / other channels. slot handles this.
			<-done
		}
	}()

	// we can discard the result, mostly for timeouts / cancels.
	_, err = waiter.Wait(ctx)
	if err != nil {
		errC <- err
	}

	logger.WithError(err).Info("hot function terminated")
	return err
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
}

func (c *container) swap(stderr io.Writer) {
	// TODO meh, maybe shouldn't bury this
	gw, ok := c.stderr.(*ghostWriter)
	if ok {
		gw.swap(stderr)
	}
}

func (c *container) Id() string                     { return c.id }
func (c *container) Command() string                { return "" }
func (c *container) Input() io.Reader               { return c.stdin }
func (c *container) Logger() (io.Writer, io.Writer) { return c.stdout, c.stderr }
func (c *container) Volumes() [][2]string           { return nil }
func (c *container) WorkDir() string                { return "" }
func (c *container) Close()                         {}
func (c *container) WriteStat(drivers.Stat)         {}
func (c *container) Image() string                  { return c.image }
func (c *container) Timeout() time.Duration         { return c.timeout }
func (c *container) EnvVars() map[string]string     { return c.env }
func (c *container) Memory() uint64                 { return c.memory * 1024 * 1024 } // convert MB
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

func (g *ghostWriter) swap(w io.Writer) {
	g.Lock()
	g.inner = w
	g.Unlock()
}

func (g *ghostWriter) Write(b []byte) (int, error) {
	// we don't need to serialize writes but swapping g.inner could be a race if unprotected
	g.Lock()
	w := g.inner
	g.Unlock()
	return w.Write(b)
}
