package runner

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/runner/common"
	"github.com/fnproject/fn/api/runner/drivers"
	"github.com/fnproject/fn/api/runner/drivers/docker"
	"github.com/fnproject/fn/api/runner/drivers/mock"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"github.com/sirupsen/logrus"
)

// TODO clean all of this up, the exposed API is huge and incohesive,
// we need 1 thing that runs 1 thing and 1 thing that runs those things;
// right now this is all the things.
type Runner struct {
	driver       drivers.Driver
	taskQueue    chan *containerTask
	flog         FuncLogger
	availableMem int64
	usedMem      int64
	usedMemMutex sync.RWMutex
	hcmgr        htfnmgr
	datastore    models.Datastore
	runListeners []RunListener

	// I made this explicit to avoid confusion. Calling Enqueue(), Start(), etc on Runner which just updates stats is very confusing.
	Stats stats
}

var (
	ErrTimeOutNoMemory = errors.New("Task timed out. No available memory.")
	ErrFullQueue       = errors.New("The runner queue is full")
	WaitMemoryTimeout  = 10 * time.Second
)

const (
	DefaultTimeout     = 30
	DefaultIdleTimeout = 30
)

func New(ctx context.Context, flog FuncLogger, ds models.Datastore) (*Runner, error) {
	// TODO: Is this really required for the container drivers? Can we remove it?
	env := common.NewEnvironment(func(e *common.Environment) {})

	// TODO: Create drivers.New(runnerConfig)
	driver, err := selectDriver("docker", env, &drivers.Config{})
	if err != nil {
		return nil, err
	}

	r := &Runner{
		driver:       driver,
		taskQueue:    make(chan *containerTask, 100),
		flog:         flog,
		availableMem: getAvailableMemory(),
		usedMem:      0,
		datastore:    ds,
	}

	go r.queueHandler(ctx)

	return r, nil
}

func (r *Runner) Wait() {
	r.Stats.Wait()
}

// This routine checks for available memory;
// If there's memory then send signal to the task to proceed.
// If there's not available memory to run the task it waits
// If the task waits for more than X seconds it timeouts
func (r *Runner) queueHandler(ctx context.Context) {
consumeQueue:
	for {
		select {
		case task := <-r.taskQueue:
			r.handleTask(task)
		case <-ctx.Done():
			break consumeQueue
		}
	}

	// consume remainders
	for len(r.taskQueue) > 0 {
		r.handleTask(<-r.taskQueue)
	}
}

func (r *Runner) handleTask(task *containerTask) {
	waitStart := time.Now()

	var waitTime time.Duration
	var timedOut bool

	// Loop waiting for available memory
	for !r.checkRequiredMem(task.cfg.Memory) {
		waitTime = time.Since(waitStart)
		if waitTime > WaitMemoryTimeout {
			timedOut = true
			break
		}
		time.Sleep(time.Microsecond)
	}

	if timedOut {
		// Send to a signal to this task saying it cannot run
		task.canRun <- false
		return
	}

	// Send a signal to this task saying it can run
	task.canRun <- true
}

func (r *Runner) hasAsyncAvailableMemory() bool {
	r.usedMemMutex.RLock()
	defer r.usedMemMutex.RUnlock()
	// reserve at least half of the memory for sync
	return (r.availableMem/2)-r.usedMem > 0
}

func (r *Runner) checkRequiredMem(req uint64) bool {
	r.usedMemMutex.RLock()
	defer r.usedMemMutex.RUnlock()
	return r.availableMem-r.usedMem-(int64(req)*1024*1024) > 0
}

func (r *Runner) addUsedMem(used int64) {
	r.usedMemMutex.Lock()
	r.usedMem = r.usedMem + used*1024*1024
	if r.usedMem < 0 {
		r.usedMem = 0
	}
	r.usedMemMutex.Unlock()
}

func (r *Runner) checkMemAndUse(req uint64) bool {
	r.usedMemMutex.Lock()
	defer r.usedMemMutex.Unlock()

	used := int64(req) * 1024 * 1024

	if r.availableMem-r.usedMem-used < 0 {
		return false
	}

	r.usedMem += used

	return true
}

func (r *Runner) awaitSlot(ctask *containerTask) error {
	span, _ := opentracing.StartSpanFromContext(ctask.ctx, "wait_mem_slot")
	defer span.Finish()
	// Check if has enough available memory
	// If available, use it
	if !r.checkMemAndUse(ctask.cfg.Memory) {
		// If not, try add task to the queue
		select {
		case r.taskQueue <- ctask:
		default:
			span.LogFields(log.Int("queue full", 1))
			// If queue is full, return error
			return ErrFullQueue
		}

		// If task was added to the queue, wait for permission
		if ok := <-ctask.canRun; !ok {
			span.LogFields(log.Int("memory timeout", 1))
			// This task timed out, not available memory
			return ErrTimeOutNoMemory
		}
	}
	return nil
}

// run is responsible for running 1 instance of a docker container
func (r *Runner) run(ctx context.Context, cfg *models.Task) (drivers.RunResult, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "run_container")
	defer span.Finish()

	if cfg.Stdout == nil {
		// TODO why? async?
		cfg.Stdout = cfg.Stderr
	}

	ctask := &containerTask{
		ctx:    ctx,
		cfg:    cfg,
		canRun: make(chan bool),
	}

	err := r.awaitSlot(ctask)
	if err != nil {
		return nil, err
	}
	defer r.addUsedMem(-1 * int64(cfg.Memory))

	span, pctx := opentracing.StartSpanFromContext(ctx, "prepare")
	cookie, err := r.driver.Prepare(pctx, ctask)
	span.Finish()
	if err != nil {
		return nil, err
	}
	defer cookie.Close(ctx)

	select {
	case <-cfg.Ready:
	default:
		close(cfg.Ready)
	}

	span, rctx := opentracing.StartSpanFromContext(ctx, "run")
	result, err := cookie.Run(rctx)
	span.Finish()
	if err != nil {
		return nil, err
	}

	span.LogFields(log.String("status", result.Status()))
	return result, nil
}

func (r Runner) EnsureImageExists(ctx context.Context, cfg *models.Task) error {
	ctask := &containerTask{
		cfg: cfg,
	}

	auth, err := ctask.DockerAuth()
	if err != nil {
		return err
	}

	_, err = docker.CheckRegistry(ctx, ctask.Image(), auth)
	return err
}

func selectDriver(driver string, env *common.Environment, conf *drivers.Config) (drivers.Driver, error) {
	switch driver {
	case "docker":
		docker := docker.NewDocker(env, *conf)
		return docker, nil
	case "mock":
		return mock.New(), nil
	}
	return nil, fmt.Errorf("driver %v not found", driver)
}

func getAvailableMemory() int64 {
	const tooBig = 322122547200 // #300GB or 0, biggest aws instance is 244GB

	var availableMemory uint64 = tooBig
	if runtime.GOOS == "linux" {
		availableMemory, err := checkCgroup()
		if err != nil {
			logrus.WithError(err).Error("Error checking for cgroup memory limits, falling back to host memory available..")
		}
		if availableMemory > tooBig || availableMemory == 0 {
			// Then -m flag probably wasn't set, so use max available on system
			availableMemory, err = checkProc()
			if err != errCantReadMemInfo &&
				(availableMemory > tooBig || availableMemory == 0) {
				logrus.WithError(err).Fatal("Cannot get the proper information to. You must specify the maximum available memory by passing the -m command with docker run when starting the runner via docker, eg:  `docker run -m 2G ...`")
			}
		}
	} else {
		// This still lets 10-20 functions execute concurrently assuming a 2GB machine.
		availableMemory = 2 * 1024 * 1024 * 1024
	}

	return int64(availableMemory)
}

func checkCgroup() (uint64, error) {
	f, err := os.Open("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	limBytes := string(b)
	limBytes = strings.TrimSpace(limBytes)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(limBytes, 10, 64)
}

var errCantReadMemInfo = errors.New("Didn't find MemAvailable in /proc/meminfo, kernel is probably < 3.14")

func checkProc() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		b := scanner.Text()
		if !strings.HasPrefix(b, "MemAvailable") {
			continue
		}

		// expect form:
		// MemAvailable: 1234567890 kB
		tri := strings.Fields(b)
		if len(tri) != 3 {
			return 0, fmt.Errorf("MemAvailable line has unexpected format: %v", b)
		}

		c, err := strconv.ParseUint(tri[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("Could not parse MemAvailable: %v", b)
		}
		switch tri[2] { // convert units to bytes
		case "kB":
			c *= 1024
		case "MB":
			c *= 1024 * 1024
		default:
			return 0, fmt.Errorf("Unexpected units for MemAvailable in /proc/meminfo, need kB or MB, got: %v", tri[2])
		}
		return c, nil
	}

	return 0, errCantReadMemInfo
}

func (r *Runner) FireBeforeRun(ctx context.Context, task *models.Task) error {
	for _, l := range r.runListeners {
		err := l.BeforeRun(ctx, task)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) FireAfterRun(ctx context.Context, task *models.Task, result drivers.RunResult) error {
	for _, l := range r.runListeners {
		err := l.AfterRun(ctx, task, result)
		if err != nil {
			return err
		}
	}
	return nil
}
