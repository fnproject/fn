package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/runner/protocol"
	"github.com/iron-io/functions/api/runner/task"
	"github.com/iron-io/runner/drivers"
)

// Hot containers - theory of operation
//
// A function is converted into a hot container if its `Format` is either
// a streamable format/protocol. At the very first task request a hot
// container shall be started and run it. Each hot container has an internal
// clock that actually halts the container if it goes idle long enough. In the
// absence of workload, it just stops the whole clockwork.
//
// Internally, the hot container uses a modified Config whose Stdin and Stdout
// are bound to an internal pipe. This internal pipe is fed with incoming tasks
// Stdin and feeds incoming tasks with Stdout.
//
// Each execution is the alternation of feeding hot containers stdin with tasks
// stdin, and reading the answer back from containers stdout. For all `Format`s
// we send embedded into the message metadata to help the container to know when
// to stop reading from its stdin and Functions expect the container to do the
// same. Refer to api/runner/protocol.go for details of these communications.
//
// Hot Containers implementation relies in two moving parts (drawn below):
// htcntrmgr and htcntr. Refer to their respective comments for
// details.
//                             │
//                         Incoming
//                           Task
//                             │
//                             ▼
//                     ┌───────────────┐
//                     │ Task Request  │
//                     │   Main Loop   │
//                     └───────────────┘
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
//       │ Container │   │ Container │   │ Container │   │ Container │
//       └───────────┘   └───────────┘   └───────────┘   └───────────┘
//                                           Timeout
//                                           Terminate
//                                           (internal clock)

const (
	// Terminate hot container after this timeout
	htcntrScaleDownTimeout = 30 * time.Second
)

// RunTask helps sending a task.Request into the common concurrency stream.
// Refer to StartWorkers() to understand what this is about.
func RunTask(tasks chan task.Request, ctx context.Context, cfg *task.Config) (drivers.RunResult, error) {
	tresp := make(chan task.Response)
	treq := task.Request{Ctx: ctx, Config: cfg, Response: tresp}
	tasks <- treq
	resp := <-treq.Response
	return resp.Result, resp.Err
}

// StartWorkers operates the common concurrency stream, ie, it will process all
// IronFunctions tasks, either sync or async. In the process, it also dispatches
// the workload to either regular or hot containers.
func StartWorkers(ctx context.Context, rnr *Runner, tasks <-chan task.Request) {
	var wg sync.WaitGroup
	defer wg.Wait()
	var hcmgr htcntrmgr

	for {
		select {
		case <-ctx.Done():
			return
		case task := <-tasks:
			p := hcmgr.getPipe(ctx, rnr, task.Config)
			if p == nil {
				wg.Add(1)
				go runTaskReq(rnr, &wg, task)
				continue
			}

			select {
			case <-ctx.Done():
				return
			case p <- task:
			}
		}
	}
}

// htcntrmgr is the intermediate between the common concurrency stream and
// hot containers. All hot containers share a single task.Request stream per
// function (chn), but each function may have more than one hot container (hc).
type htcntrmgr struct {
	chn map[string]chan task.Request
	hc  map[string]*htcntrsvr
}

func (h *htcntrmgr) getPipe(ctx context.Context, rnr *Runner, cfg *task.Config) chan task.Request {
	isStream, err := protocol.IsStreamable(cfg.Format)
	if err != nil {
		logrus.WithError(err).Info("could not detect container IO protocol")
		return nil
	} else if !isStream {
		return nil
	}

	if h.chn == nil {
		h.chn = make(map[string]chan task.Request)
		h.hc = make(map[string]*htcntrsvr)
	}

	// TODO(ccirello): re-implement this without memory allocation (fmt.Sprint)
	fn := fmt.Sprint(cfg.AppName, ",", cfg.Path, cfg.Image, cfg.Timeout, cfg.Memory, cfg.Format, cfg.MaxConcurrency)
	tasks, ok := h.chn[fn]
	if !ok {
		h.chn[fn] = make(chan task.Request)
		tasks = h.chn[fn]
		svr := newhtcntrsvr(ctx, cfg, rnr, tasks)
		if err := svr.launch(ctx); err != nil {
			logrus.WithError(err).Error("cannot start hot container supervisor")
			return nil
		}
		h.hc[fn] = svr
	}

	return tasks
}

// htcntrsvr is part of htcntrmgr, abstracted apart for simplicity, its only
// purpose is to test for hot containers saturation and try starting as many as
// needed. In case of absence of workload, it will stop trying to start new hot
// containers.
type htcntrsvr struct {
	cfg      *task.Config
	rnr      *Runner
	tasksin  <-chan task.Request
	tasksout chan task.Request
	maxc     chan struct{}
}

func newhtcntrsvr(ctx context.Context, cfg *task.Config, rnr *Runner, tasks <-chan task.Request) *htcntrsvr {
	svr := &htcntrsvr{
		cfg:      cfg,
		rnr:      rnr,
		tasksin:  tasks,
		tasksout: make(chan task.Request, 1),
		maxc:     make(chan struct{}, cfg.MaxConcurrency),
	}

	// This pipe will take all incoming tasks and just forward them to the
	// started hot containers. The catch here is that it feeds a buffered
	// channel from an unbuffered one. And this buffered channel is
	// then used to determine the presence of running hot containers.
	// If no hot container is available, tasksout will fill up to its
	// capacity and pipe() will start them.
	go svr.pipe(ctx)
	return svr
}

func (svr *htcntrsvr) pipe(ctx context.Context) {
	for {
		select {
		case t := <-svr.tasksin:
			svr.tasksout <- t
			if len(svr.tasksout) > 0 {
				if err := svr.launch(ctx); err != nil {
					logrus.WithError(err).Error("cannot start more hot containers")
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (svr *htcntrsvr) launch(ctx context.Context) error {
	select {
	case svr.maxc <- struct{}{}:
		hc, err := newhtcntr(
			svr.cfg,
			protocol.Protocol(svr.cfg.Format),
			svr.tasksout,
			svr.rnr,
		)
		if err != nil {
			return err
		}
		go func() {
			hc.serve(ctx)
			<-svr.maxc
		}()
	default:
	}

	return nil
}

// htcntr actually interfaces an incoming task from the common concurrency
// stream into a long lived container. If idle long enough, it will stop. It
// uses route configuration to determine which protocol to use.
type htcntr struct {
	cfg   *task.Config
	proto protocol.ContainerIO
	tasks <-chan task.Request

	// Side of the pipe that takes information from outer world
	// and injects into the container.
	in  io.Writer
	out io.Reader

	// Receiving side of the container.
	containerIn  io.Reader
	containerOut io.Writer

	rnr *Runner
}

func newhtcntr(cfg *task.Config, proto protocol.Protocol, tasks <-chan task.Request, rnr *Runner) (*htcntr, error) {
	stdinr, stdinw := io.Pipe()
	stdoutr, stdoutw := io.Pipe()

	p, err := protocol.New(proto, stdinw, stdoutr)
	if err != nil {
		return nil, err
	}

	hc := &htcntr{
		cfg:   cfg,
		proto: p,
		tasks: tasks,

		in:  stdinw,
		out: stdoutr,

		containerIn:  stdinr,
		containerOut: stdoutw,

		rnr: rnr,
	}

	return hc, nil
}

func (hc *htcntr) serve(ctx context.Context) {
	lctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			inactivity := time.After(htcntrScaleDownTimeout)

			select {
			case <-lctx.Done():
				return

			case <-inactivity:
				cancel()

			case t := <-hc.tasks:
				if err := hc.proto.Dispatch(lctx, t); err != nil {
					logrus.WithField("ctx", lctx).Info("task failed")
					t.Response <- task.Response{
						&runResult{StatusValue: "error", error: err},
						err,
					}
					continue
				}

				t.Response <- task.Response{
					&runResult{StatusValue: "success"},
					nil,
				}
			}
		}
	}()

	cfg := *hc.cfg
	cfg.Timeout = 0 // add a timeout to simulate ab.end. failure.
	cfg.Stdin = hc.containerIn
	cfg.Stdout = hc.containerOut

	// Why can we not attach stderr to the task like we do for stdin and
	// stdout?
	//
	// Stdin/Stdout are completely known to the scope of the task. You must
	// have a task stdin to feed containers stdin, and also the other way
	// around when reading from stdout. So both are directly related to the
	// life cycle of the request.
	//
	// Stderr, on the other hand, can be written by anything any time:
	// failure between requests, failures inside requests and messages send
	// right after stdout has been finished being transmitted. Thus, with
	// hot containers, there is not a 1:1 relation between stderr and tasks.
	//
	// Still, we do pass - at protocol level - a Task-ID header, from which
	// the application running inside the hot container can use to identify
	// its own stderr output.
	errr, errw := io.Pipe()
	cfg.Stderr = errw
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(errr)
		for scanner.Scan() {
			logrus.WithFields(logrus.Fields{
				"app":             cfg.AppName,
				"route":           cfg.Path,
				"image":           cfg.Image,
				"memory":          cfg.Memory,
				"format":          cfg.Format,
				"max_concurrency": cfg.MaxConcurrency,
			}).Info(scanner.Text())
		}
	}()

	result, err := hc.rnr.Run(lctx, &cfg)
	if err != nil {
		logrus.WithError(err).Error("hot container failure detected")
	}
	cancel()
	errw.Close()
	wg.Wait()
	logrus.WithField("result", result).Info("hot container terminated")
}

func runTaskReq(rnr *Runner, wg *sync.WaitGroup, t task.Request) {
	defer wg.Done()
	result, err := rnr.Run(t.Ctx, t.Config)
	select {
	case t.Response <- task.Response{result, err}:
		close(t.Response)
	default:
	}
}

type runResult struct {
	error
	StatusValue string
}

func (r *runResult) Error() string {
	if r.error == nil {
		return ""
	}
	return r.error.Error()
}

func (r *runResult) Status() string { return r.StatusValue }
