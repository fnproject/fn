package runner

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/drivers"
	"github.com/fnproject/fn/api/runner/protocol"
	"github.com/fnproject/fn/api/runner/task"
	"github.com/go-openapi/strfmt"
	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
)

// hot functions - theory of operation
//
// A function is converted into a hot function if its `Format` is either
// a streamable format/protocol. At the very first task request a hot
// container shall be started and run it. Each hot function has an internal
// clock that actually halts the container if it goes idle long enough. In the
// absence of workload, it just stops the whole clockwork.
//
// Internally, the hot function uses a modified Config whose Stdin and Stdout
// are bound to an internal pipe. This internal pipe is fed with incoming tasks
// Stdin and feeds incoming tasks with Stdout.
//
// Each execution is the alternation of feeding hot functions stdin with tasks
// stdin, and reading the answer back from containers stdout. For all `Format`s
// we send embedded into the message metadata to help the container to know when
// to stop reading from its stdin and Functions expect the container to do the
// same. Refer to api/runner/protocol.go for details of these communications.
//
// hot functions implementation relies in two moving parts (drawn below):
// htfnmgr and htfn. Refer to their respective comments for
// details.
//                             │
//                         Incoming
//                           Task
//                             │
//                      ┌──────▼────────┐
//                     ┌┴──────────────┐│
//                     │  Per Function ││             non-streamable f()
//             ┌───────│   Container   │├──────┐───────────────┐
//             │       │    Manager    ├┘      │               │
//             │       └───────────────┘       │               │
//             │               │               │               │
//             ▼               ▼               ▼               ▼
//       ┌───────────┐   ┌───────────┐   ┌───────────┐   ┌───────────┐
//       │    Hot    │   │    Hot    │   │    Hot    │   │   Cold    │
//       │ Function  │   │ Function  │   │ Function  │   │ Function  │
//       └───────────┘   └───────────┘   └───────────┘   └───────────┘
//                                           Timeout
//                                           Terminate
//                                           (internal clock)

// RunTrackedTask is just a wrapper for shared logic for async/sync runners
func (rnr *Runner) RunTrackedTask(newTask *models.Task, ctx context.Context, cfg *task.Config) (drivers.RunResult, error) {
	startedAt := strfmt.DateTime(time.Now())
	newTask.StartedAt = startedAt

	result, err := rnr.RunTask(ctx, cfg)

	completedAt := strfmt.DateTime(time.Now())
	status := "error"
	if result != nil {
		status = result.Status()
	}
	newTask.CompletedAt = completedAt
	newTask.Status = status

	if err := rnr.datastore.InsertTask(ctx, newTask); err != nil {
		// TODO we should just log this error not return it to user? just issue storing task status but task is run
		logrus.WithError(err).Error("error inserting task into datastore")
	}

	return result, err
}

// RunTask will dispatch a task specified by cfg to a hot container, if possible,
// that already exists or will create a new container to run a task and then run it.
// TODO XXX (reed): merge this and RunTrackedTask to reduce surface area...
func (rnr *Runner) RunTask(ctx context.Context, cfg *task.Config) (drivers.RunResult, error) {
	rnr.Start() // TODO layering issue ???
	defer rnr.Complete()

	tresp := make(chan task.Response)
	treq := task.Request{Ctx: ctx, Config: cfg, Response: tresp}
	tasks := rnr.hcmgr.getPipe(ctx, rnr, cfg)
	if tasks == nil {
		// TODO get rid of this to use herd stuff
		go runTaskReq(rnr, treq)
	} else {
		tasks <- treq
	}

	resp := <-treq.Response
	if resp.Result == nil && resp.Err == nil {
		resp.Err = errors.New("error running task with unknown error")
	}
	return resp.Result, resp.Err
}

// htfnmgr tracks all hot functions, used to funnel kittens into existing tubes
// XXX (reed): this map grows unbounded, need to add LRU but need to make
// sure that no functions are running when we evict
type htfnmgr struct {
	sync.RWMutex
	hc map[string]*htfnsvr
}

func (h *htfnmgr) getPipe(ctx context.Context, rnr *Runner, cfg *task.Config) chan<- task.Request {
	isStream := protocol.IsStreamable(protocol.Protocol(cfg.Format))
	if !isStream {
		// TODO stop doing this, to prevent herds
		return nil
	}

	h.RLock()
	if h.hc == nil {
		h.RUnlock()
		h.Lock()
		if h.hc == nil {
			h.hc = make(map[string]*htfnsvr)
		}
		h.Unlock()
		h.RLock()
	}

	fn := key(cfg)

	svr, ok := h.hc[fn]
	h.RUnlock()
	if !ok {
		h.Lock()
		svr, ok = h.hc[fn]
		if !ok {
			svr = newhtfnsvr(ctx, cfg, rnr)
			h.hc[fn] = svr
		}
		h.Unlock()
	}

	return svr.tasksin
}

func key(cfg *task.Config) string {
	// return a sha1 hash of a (hopefully) unique string of all the config
	// values, to make map lookups quicker [than the giant unique string]

	hash := sha1.New()
	fmt.Fprint(hash, cfg.AppName, "\x00")
	fmt.Fprint(hash, cfg.Path, "\x00")
	fmt.Fprint(hash, cfg.Image, "\x00")
	fmt.Fprint(hash, cfg.Timeout, "\x00")
	fmt.Fprint(hash, cfg.IdleTimeout, "\x00")
	fmt.Fprint(hash, cfg.Memory, "\x00")
	fmt.Fprint(hash, cfg.Format, "\x00")

	// we have to sort these before printing, yay. TODO do better
	keys := make([]string, 0, len(cfg.BaseEnv))
	for k := range cfg.BaseEnv {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprint(hash, k, "\x00", cfg.BaseEnv[k], "\x00")
	}

	var buf [sha1.Size]byte
	return string(hash.Sum(buf[:0]))
}

// htfnsvr is part of htfnmgr, abstracted apart for simplicity, its only
// purpose is to test for hot functions saturation and try starting as many as
// needed. In case of absence of workload, it will stop trying to start new hot
// containers.
type htfnsvr struct {
	cfg *task.Config
	rnr *Runner
	// TODO sharing with only a channel among hot containers will result in
	// inefficient recycling of containers, we need a stack not a queue, so that
	// when a lot of hot containers are up and throughput drops they don't all
	// find a task every few seconds and stay up for a lot longer than we really
	// need them.
	tasksin  chan task.Request
	tasksout chan task.Request
	first    chan struct{}
	once     sync.Once // TODO this really needs to happen any time runner count goes to 0
}

func newhtfnsvr(ctx context.Context, cfg *task.Config, rnr *Runner) *htfnsvr {
	svr := &htfnsvr{
		cfg:      cfg,
		rnr:      rnr,
		tasksin:  make(chan task.Request),
		tasksout: make(chan task.Request, 1),
		first:    make(chan struct{}, 1),
	}
	svr.first <- struct{}{} // prime so that 1 thread will start the first container, others will wait

	// This pipe will take all incoming tasks and just forward them to the
	// started hot functions. The catch here is that it feeds a buffered
	// channel from an unbuffered one. And this buffered channel is
	// then used to determine the presence of running hot functions.
	// If no hot function is available, tasksout will fill up to its
	// capacity and pipe() will start them.
	go svr.pipe(context.Background()) // XXX (reed): real context for adding consuela
	return svr
}

func (svr *htfnsvr) pipe(ctx context.Context) {
	for {
		select {
		case t := <-svr.tasksin:
			svr.tasksout <- t

			// TODO move checking for ram up here? then we can wait for hot functions to open up instead of always
			// trying to make new ones if all hot functions are busy (and if machine is full and all functions are
			// hot then most new hot functions are going to time out waiting to get available ram)
			// TODO need to add some kind of metering here, we could track average run time and # of runners
			select {
			case _, ok := <-svr.first: // wait for >= 1 to be up to avoid herd
				if ok || len(svr.tasksout) > 0 {
					svr.launch(ctx)
				}
			case <-ctx.Done(): // TODO we should prob watch the task timeout not just the pipe...
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (svr *htfnsvr) launch(ctx context.Context) {
	hc := newhtfn(
		svr.cfg,
		svr.tasksout,
		svr.rnr,
		func() { svr.once.Do(func() { close(svr.first) }) },
	)
	go hc.serve(ctx)
}

// htfn is one instance of a hot container, which may or may not be running a
// task. If idle long enough, it will stop. It uses route configuration to
// determine which protocol to use.
type htfn struct {
	id    string
	cfg   *task.Config
	proto protocol.ContainerIO
	tasks <-chan task.Request
	once  func()

	// Receiving side of the container.
	containerIn  io.Reader
	containerOut io.Writer

	rnr *Runner
}

func newhtfn(cfg *task.Config, tasks <-chan task.Request, rnr *Runner, once func()) *htfn {
	stdinr, stdinw := io.Pipe()
	stdoutr, stdoutw := io.Pipe()

	return &htfn{
		id:    id.New().String(),
		cfg:   cfg,
		proto: protocol.New(protocol.Protocol(cfg.Format), stdinw, stdoutr),
		tasks: tasks,
		once:  once,

		containerIn:  stdinr,
		containerOut: stdoutw,

		rnr: rnr,
	}
}

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

func (g *ghostWriter) Close() error { return nil }

func (hc *htfn) serve(ctx context.Context) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "run_hot_container")
	defer span.Finish()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cfg := *hc.cfg
	logger := logrus.WithFields(logrus.Fields{"hot_id": hc.id, "app": cfg.AppName, "route": cfg.Path, "image": cfg.Image, "memory": cfg.Memory, "format": cfg.Format, "idle_timeout": cfg.IdleTimeout})

	// TODO go through FuncLogger probably
	// if there's no active call, log any errors to stderr (for debugging issues)
	bwLog := newLineWriter(&logWriter{ctx: ctx, appName: cfg.AppName, path: cfg.Path, image: cfg.Image, reqID: hc.id})
	defer bwLog.Close()

	stderr := &ghostWriter{inner: bwLog}

	first := true
	go func() {
		for {
			select {
			case <-ctx.Done():
			case <-cfg.Ready:
				// on first execution, wait before starting idle timeout / stopping wait time clock,
				// since docker pull / container create need to happen.
				// XXX (reed): should we still obey the task timeout? docker image could be 8GB...
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(cfg.IdleTimeout):
				logger.Info("Canceling inactive hot function")
				cancel()
			case t := <-hc.tasks:
				var span opentracing.Span
				if first {
					// TODO this doesn't work as intended; beyond me atm, but the spans do come up.
					// need a way to add the span from starting container to the first execution, basically.
					spanHot := opentracing.SpanFromContext(ctx)
					spanTask := opentracing.SpanFromContext(t.Ctx)
					span = opentracing.StartSpan("dispatch", opentracing.ChildOf(spanTask.Context()), opentracing.FollowsFrom(spanHot.Context()))
					ctx = opentracing.ContextWithSpan(t.Ctx, span)
					first = false
				} else {
					span, ctx = opentracing.StartSpanFromContext(t.Ctx, "dispatch")
				}

				// swap logs to log to the task logger instead of stderr
				tlog := hc.rnr.flog.Writer(ctx, cfg.AppName, cfg.Path, cfg.Image, cfg.ID)
				stderr.swap(tlog)

				start := time.Now()
				err := hc.proto.Dispatch(ctx, t.Config)
				status := "success"
				if err != nil {
					status = "error"
					logrus.WithField("ctx", ctx).Info("task failed")
				}
				span.Finish()
				hc.once()

				stderr.swap(bwLog) // swap back out before flush
				tlog.Close()       // write to db/flush

				t.Response <- task.Response{
					Result: &runResult{start: start, status: status, error: err},
					Err:    err,
				}
			}
		}
	}()

	cfg.Timeout = 0 // add a timeout to simulate ab.end. failure.
	cfg.Stdin = hc.containerIn
	cfg.Stdout = hc.containerOut
	cfg.Stderr = stderr

	// TODO how to tie a span from the first task into this? yikes
	result, err := hc.rnr.run(ctx, &cfg)
	if err != nil {
		logger.WithError(err).Error("hot function failure detected")
	}
	logger.WithField("result", result).Info("hot function terminated")
}

// TODO make Default protocol a real thing and get rid of this in favor of Dispatch
func runTaskReq(rnr *Runner, t task.Request) {
	// TODO this will not be such a shit storm after the above TODO is TODONE
	cfg := t.Config
	t.Config.Stderr = rnr.flog.Writer(t.Ctx, cfg.AppName, cfg.Path, cfg.Image, cfg.ID)
	defer t.Config.Stderr.Close()

	result, err := rnr.run(t.Ctx, t.Config)
	select {
	case t.Response <- task.Response{result, err}:
		close(t.Response)
	default:
	}
}

type runResult struct {
	error
	status string
	start  time.Time
}

func (r *runResult) Error() string {
	if r.error == nil {
		return ""
	}
	return r.error.Error()
}

func (r *runResult) Status() string       { return r.status }
func (r *runResult) StartTime() time.Time { return r.start }
