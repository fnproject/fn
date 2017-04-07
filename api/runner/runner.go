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

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/functions/api/runner/task"
	"github.com/iron-io/runner/common"
	"github.com/iron-io/runner/drivers"
	driverscommon "github.com/iron-io/runner/drivers"
	"github.com/iron-io/runner/drivers/docker"
	"github.com/iron-io/runner/drivers/mock"
)

type Runner struct {
	driver       drivers.Driver
	taskQueue    chan *containerTask
	mlog         MetricLogger
	flog         FuncLogger
	availableMem int64
	usedMem      int64
	usedMemMutex sync.RWMutex

	stats
}

var (
	ErrTimeOutNoMemory = errors.New("Task timed out. No available memory.")
	ErrFullQueue       = errors.New("The runner queue is full")

	WaitMemoryTimeout = 10 * time.Second
)

func New(ctx context.Context, flog FuncLogger, mlog MetricLogger) (*Runner, error) {
	// TODO: Is this really required for the container drivers? Can we remove it?
	env := common.NewEnvironment(func(e *common.Environment) {})

	// TODO: Create a drivers.New(runnerConfig) in Titan
	driver, err := selectDriver("docker", env, &driverscommon.Config{})
	if err != nil {
		return nil, err
	}

	r := &Runner{
		driver:       driver,
		taskQueue:    make(chan *containerTask, 100),
		flog:         flog,
		mlog:         mlog,
		availableMem: getAvailableMemory(),
		usedMem:      0,
	}

	go r.queueHandler(ctx)

	return r, nil
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

	metricBaseName := fmt.Sprintf("run.%s.", task.cfg.AppName)
	r.mlog.LogTime(task.ctx, metricBaseName+"wait_time", waitTime)
	r.mlog.LogTime(task.ctx, "run.wait_time", waitTime)

	if timedOut {
		// Send to a signal to this task saying it cannot run
		r.mlog.LogCount(task.ctx, metricBaseName+"timeout", 1)
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
	return (r.availableMem-r.usedMem)/int64(req)*1024*1024 > 0
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

	if (r.availableMem-r.usedMem)/used < 0 {
		return false
	}

	r.usedMem += used

	return true
}

func (r *Runner) Run(ctx context.Context, cfg *task.Config) (drivers.RunResult, error) {
	var err error

	if cfg.Memory == 0 {
		cfg.Memory = 128
	}

	cfg.Stderr = r.flog.Writer(ctx, cfg.AppName, cfg.Path, cfg.Image, cfg.ID)
	if cfg.Stdout == nil {
		cfg.Stdout = cfg.Stderr
	}

	ctask := &containerTask{
		ctx:    ctx,
		cfg:    cfg,
		canRun: make(chan bool),
	}

	metricBaseName := fmt.Sprintf("run.%s.", cfg.AppName)
	r.mlog.LogCount(ctx, metricBaseName+"requests", 1)

	// Check if has enough available memory
	// If available, use it
	if !r.checkMemAndUse(cfg.Memory) {
		// If not, try add task to the queue
		select {
		case r.taskQueue <- ctask:
		default:
			// If queue is full, return error
			r.mlog.LogCount(ctx, "queue.full", 1)
			return nil, ErrFullQueue
		}

		// If task was added to the queue, wait for permission
		if ok := <-ctask.canRun; !ok {
			// This task timed out, not available memory
			return nil, ErrTimeOutNoMemory
		}
	} else {
		r.mlog.LogTime(ctx, metricBaseName+"waittime", 0)
	}
	defer r.addUsedMem(-1 * int64(cfg.Memory))

	cookie, err := r.driver.Prepare(ctx, ctask)
	if err != nil {
		return nil, err
	}
	defer cookie.Close()

	metricStart := time.Now()

	result, err := cookie.Run(ctx)
	if err != nil {
		return nil, err
	}

	if result.Status() == "success" {
		r.mlog.LogCount(ctx, metricBaseName+"succeeded", 1)
	} else {
		r.mlog.LogCount(ctx, metricBaseName+"error", 1)
	}

	metricElapsed := time.Since(metricStart)
	r.mlog.LogTime(ctx, metricBaseName+"time", metricElapsed)
	r.mlog.LogTime(ctx, "run.exec_time", metricElapsed)

	return result, nil
}

func (r Runner) EnsureImageExists(ctx context.Context, cfg *task.Config) error {
	ctask := &containerTask{
		cfg: cfg,
	}

	auth, err := ctask.DockerAuth()
	if err != nil {
		return err
	}

	_, err = docker.CheckRegistry(ctask.Image(), auth)
	return err
}

func selectDriver(driver string, env *common.Environment, conf *driverscommon.Config) (drivers.Driver, error) {
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
