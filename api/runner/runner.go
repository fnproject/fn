package runner

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/iron-io/titan/common"
	"github.com/iron-io/titan/runner/agent"
	"github.com/iron-io/titan/runner/drivers"
	driverscommon "github.com/iron-io/titan/runner/drivers"
	"github.com/iron-io/titan/runner/drivers/docker"
	"github.com/iron-io/titan/runner/drivers/mock"
)

type Config struct {
	ID      string
	Image   string
	Timeout time.Duration
	AppName string
	Memory  uint64
	Env     map[string]string
	Stdout  io.Writer
	Stderr  io.Writer
}

type Runner struct {
	driver    drivers.Driver
	taskQueue chan *containerTask
	ml        Logger
}

var (
	ErrTimeOutNoMemory = errors.New("Task timed out. No available memory.")
	ErrFullQueue       = errors.New("The runner queue is full")

	WaitMemoryTimeout = 10 * time.Second
)

func New(metricLogger Logger) (*Runner, error) {
	// TODO: Is this really required for Titan's driver?
	// Can we remove it?
	env := common.NewEnvironment(func(e *common.Environment) {})

	// TODO: Create a drivers.New(runnerConfig) in Titan
	driver, err := selectDriver("docker", env, &driverscommon.Config{})
	if err != nil {
		return nil, err
	}

	r := &Runner{
		driver:    driver,
		taskQueue: make(chan *containerTask, 100),
		ml:        metricLogger,
	}

	go r.queueHandler()

	return r, nil
}

// This routine checks for available memory;
// If there's memory then send signal to the task to proceed.
// If there's not available memory to run the task it waits
// If the task waits for more than X seconds it timeouts
func (r *Runner) queueHandler() {
	var task *containerTask
	var waitStart time.Time
	var waitTime time.Duration
	var timedOut bool
	for {
		select {
		case task = <-r.taskQueue:
			waitStart = time.Now()
			timedOut = false
		}

		// Loop waiting for available memory
		avail := dynamicSizing(task.cfg.Memory)
		for ; avail == 0; avail = dynamicSizing(task.cfg.Memory) {
			waitTime = time.Since(waitStart)
			if waitTime > WaitMemoryTimeout {
				timedOut = true
				break
			}
		}

		metricBaseName := fmt.Sprintf("run.%s.", task.cfg.AppName)
		r.ml.Log(task.ctx, Metric{"name": (metricBaseName + "waittime"), "type": "time", "value": waitTime})

		if timedOut {
			// Send to a signal to this task saying it cannot run
			r.ml.Log(task.ctx, Metric{"name": (metricBaseName + "timeout"), "type": "count", "value": 1})
			task.canRun <- false
			continue
		}

		// Send a signal to this task saying it can run
		task.canRun <- true
	}
}

func (r *Runner) Run(ctx context.Context, cfg *Config) (drivers.RunResult, error) {
	var err error

	ctask := &containerTask{
		ctx:    ctx,
		cfg:    cfg,
		auth:   &agent.ConfigAuth{},
		canRun: make(chan bool),
	}

	metricBaseName := fmt.Sprintf("run.%s.", cfg.AppName)
	r.ml.Log(ctx, Metric{"name": (metricBaseName + "requests"), "type": "count", "value": 1})

	closer, err := r.driver.Prepare(ctx, ctask)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// Check if has enough available memory
	if dynamicSizing(cfg.Memory) == 0 {
		// If not, try add task to the queue
		select {
		case r.taskQueue <- ctask:
		default:
			// If queue is full, return error
			r.ml.Log(ctx, Metric{"name": "queue.full", "type": "count", "value": 1})
			return nil, ErrFullQueue
		}

		// If task was added to the queue, wait for permission
		if ok := <-ctask.canRun; !ok {
			// This task timed out, not available memory
			return nil, ErrTimeOutNoMemory
		}
	} else {
		r.ml.Log(ctx, Metric{"name": (metricBaseName + "waittime"), "type": "time", "value": 0})
	}

	metricStart := time.Now()
	result, err := r.driver.Run(ctx, ctask)
	if err != nil {
		return nil, err
	}

	if result.Status() == "success" {
		r.ml.Log(ctx, Metric{"name": (metricBaseName + "succeeded"), "type": "count", "value": 1})
	} else {
		r.ml.Log(ctx, Metric{"name": (metricBaseName + "error"), "type": "count", "value": 1})
	}

	metricElapsed := time.Since(metricStart)
	r.ml.Log(ctx, Metric{"name": (metricBaseName + "time"), "type": "time", "value": metricElapsed})

	return result, nil
}

func (r Runner) EnsureImageExists(ctx context.Context, cfg *Config) error {
	ctask := &containerTask{
		cfg:  cfg,
		auth: &agent.ConfigAuth{},
	}

	err := r.driver.EnsureImageExists(ctx, ctask)
	if err != nil {
		return err
	}
	return nil
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

func dynamicSizing(reqMem uint64) int {
	const tooBig = 322122547200 // #300GB or 0, biggest aws instance is 244GB
	if reqMem == 0 {
		reqMem = 128
	}

	availableMemory, err := checkCgroup()
	if err != nil {
		logrus.WithError(err).Error("Error checking for cgroup memory limits, falling back to host memory available..")
	}
	if availableMemory > tooBig || availableMemory == 0 {
		// Then -m flag probably wasn't set, so use max available on system
		availableMemory, err = checkProc()
		if availableMemory > tooBig || availableMemory == 0 {
			logrus.WithError(err).Fatal("Your Linux version is too old (<3.14) then we can't get the proper information to . You must specify the maximum available memory by passing the -m command with docker run when starting the runner via docker, eg:  `docker run -m 2G ...`")
		}
	}

	c := availableMemory / (reqMem * 1024 * 1024)

	return int(c)
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

	return 0, fmt.Errorf("Didn't find MemAvailable in /proc/meminfo, kernel is probably < 3.14")
}
