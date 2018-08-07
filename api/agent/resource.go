package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"go.opencensus.io/trace"

	"github.com/sirupsen/logrus"
)

const (
	Mem1MB = 1024 * 1024
	Mem1GB = 1024 * 1024 * 1024

	// Assume 2GB RAM on non-linux systems
	DefaultNonLinuxMemory = 2048 * Mem1MB
)

var CapacityFull = errors.New("max capacity reached")

// A simple resource (memory, cpu, disk, etc.) tracker for scheduling.
// TODO: add cpu, disk, network IO for future
type ResourceTracker interface {
	// WaitAsyncResource returns a channel that will send once when there seem to be sufficient
	// resource levels to run an async task, it is up to the implementer to create policy here.
	WaitAsyncResource(ctx context.Context) chan struct{}

	// GetResourceToken returns a channel to wait for a resource token on. If the provided context is canceled,
	// the channel will never receive anything. If it is not possible to fulfill this resource, the channel
	// will never receive anything (use IsResourcePossible). If a resource token is available for the provided
	// resource parameters, it will otherwise be sent once on the returned channel. The channel is never closed.
	// if isNB is set, resource check is done and error token is returned without blocking.
	// if isAsync is set, resource allocation specific for async requests is considered. (eg. always allow
	// a sync only reserve area) Memory is expected to be provided in MB units.
	GetResourceToken(ctx context.Context, memory, cpuQuota uint64, isAsync, isNB bool) <-chan ResourceToken

	// IsResourcePossible returns whether it's possible to fulfill the requested resources on this
	// machine. It must be called before GetResourceToken or GetResourceToken may hang.
	// Memory is expected to be provided in MB units.
	IsResourcePossible(memory, cpuQuota uint64, isAsync bool) bool
}

type resourceTracker struct {
	// cond protects access to ram variables below
	cond *sync.Cond
	// ramTotal is the total usable memory for sync functions
	ramSyncTotal uint64
	// ramSyncUsed is ram reserved for running sync containers including hot/idle
	ramSyncUsed uint64
	// ramAsyncTotal is the total usable memory for async + sync functions
	ramAsyncTotal uint64
	// ramAsyncUsed is ram reserved for running async + sync containers including hot/idle
	ramAsyncUsed uint64
	// memory in use for async area in which agent stops dequeuing async jobs
	ramAsyncHWMark uint64

	// cpuTotal is the total usable cpu for sync functions
	cpuSyncTotal uint64
	// cpuSyncUsed is cpu reserved for running sync containers including hot/idle
	cpuSyncUsed uint64
	// cpuAsyncTotal is the total usable cpu for async + sync functions
	cpuAsyncTotal uint64
	// cpuAsyncUsed is cpu reserved for running async + sync containers including hot/idle
	cpuAsyncUsed uint64
	// cpu in use for async area in which agent stops dequeuing async jobs
	cpuAsyncHWMark uint64
}

func NewResourceTracker(cfg *Config) ResourceTracker {

	obj := &resourceTracker{
		cond: sync.NewCond(new(sync.Mutex)),
	}

	obj.initializeMemory(cfg)
	obj.initializeCPU(cfg)
	return obj
}

type ResourceToken interface {
	// Close must be called by any thread that receives a token.
	io.Closer
	Error() error
}

type resourceToken struct {
	once      sync.Once
	err       error
	decrement func()
}

func (t *resourceToken) Error() error {
	return t.err
}

func (t *resourceToken) Close() error {
	t.once.Do(func() {
		t.decrement()
	})
	return nil
}

func (a *resourceTracker) isResourceAvailableLocked(memory uint64, cpuQuota uint64, isAsync bool) bool {

	asyncAvailMem := a.ramAsyncTotal - a.ramAsyncUsed
	syncAvailMem := a.ramSyncTotal - a.ramSyncUsed

	asyncAvailCPU := a.cpuAsyncTotal - a.cpuAsyncUsed
	syncAvailCPU := a.cpuSyncTotal - a.cpuSyncUsed

	// For sync functions, we can steal from async pool. For async, we restrict it to sync pool
	if isAsync {
		return asyncAvailMem >= memory && asyncAvailCPU >= cpuQuota
	} else {
		return asyncAvailMem+syncAvailMem >= memory && asyncAvailCPU+syncAvailCPU >= cpuQuota
	}
}

// is this request possible to meet? If no, fail quick
func (a *resourceTracker) IsResourcePossible(memory uint64, cpuQuota uint64, isAsync bool) bool {
	memory = memory * Mem1MB

	if isAsync {
		return memory <= a.ramAsyncTotal && cpuQuota <= a.cpuAsyncTotal
	} else {
		return memory <= a.ramSyncTotal+a.ramAsyncTotal && cpuQuota <= a.cpuSyncTotal+a.cpuAsyncTotal
	}
}

func (a *resourceTracker) allocResourcesLocked(memory, cpuQuota uint64, isAsync bool) ResourceToken {

	var asyncMem, syncMem uint64
	var asyncCPU, syncCPU uint64

	if isAsync {
		// async uses async pool only
		asyncMem = memory
		asyncCPU = cpuQuota
	} else {
		// if sync fits async + sync pool
		syncMem = minUint64(a.ramSyncTotal-a.ramSyncUsed, memory)
		syncCPU = minUint64(a.cpuSyncTotal-a.cpuSyncUsed, cpuQuota)

		asyncMem = memory - syncMem
		asyncCPU = cpuQuota - syncCPU
	}

	a.ramAsyncUsed += asyncMem
	a.ramSyncUsed += syncMem
	a.cpuAsyncUsed += asyncCPU
	a.cpuSyncUsed += syncCPU

	return &resourceToken{decrement: func() {

		a.cond.L.Lock()
		a.ramAsyncUsed -= asyncMem
		a.ramSyncUsed -= syncMem
		a.cpuAsyncUsed -= asyncCPU
		a.cpuSyncUsed -= syncCPU
		a.cond.L.Unlock()

		// WARNING: yes, we wake up everyone even async waiters when only sync pool has space, but
		// the cost of this spurious wake up is unlikely to impact much performance. Simpler
		// to use one cond variable for the time being.
		a.cond.Broadcast()
	}}
}

func (a *resourceTracker) getResourceTokenNB(memory uint64, cpuQuota uint64, isAsync bool) ResourceToken {
	if !a.IsResourcePossible(memory, cpuQuota, isAsync) {
		return &resourceToken{decrement: func() {}, err: CapacityFull}
	}
	memory = memory * Mem1MB

	var t ResourceToken

	a.cond.L.Lock()

	if !a.isResourceAvailableLocked(memory, cpuQuota, isAsync) {
		t = &resourceToken{decrement: func() {}, err: CapacityFull}
	} else {
		t = a.allocResourcesLocked(memory, cpuQuota, isAsync)
	}

	a.cond.L.Unlock()
	return t
}

func (a *resourceTracker) getResourceTokenNBChan(ctx context.Context, memory uint64, cpuQuota uint64, isAsync bool) <-chan ResourceToken {
	ctx, span := trace.StartSpan(ctx, "agent_get_resource_token_nbio_chan")

	ch := make(chan ResourceToken)
	go func() {
		defer span.End()
		t := a.getResourceTokenNB(memory, cpuQuota, isAsync)

		select {
		case ch <- t:
		case <-ctx.Done():
			// if we can't send b/c nobody is waiting anymore, need to decrement here
			t.Close()
		}
	}()

	return ch
}

// the received token should be passed directly to launch (unconditionally), launch
// will close this token (i.e. the receiver should not call Close)
func (a *resourceTracker) GetResourceToken(ctx context.Context, memory uint64, cpuQuota uint64, isAsync, isNB bool) <-chan ResourceToken {
	if isNB {
		return a.getResourceTokenNBChan(ctx, memory, cpuQuota, isAsync)
	}

	ch := make(chan ResourceToken)

	if !a.IsResourcePossible(memory, cpuQuota, isAsync) {
		// return the channel, but never send anything.
		return ch
	}

	c := a.cond
	isWaiting := false

	memory = memory * Mem1MB

	// if we find a resource token, shut down the thread waiting on ctx finish.
	// alternatively, if the ctx is done, wake up the cond loop.
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		c.L.Lock()
		if isWaiting {
			c.Broadcast()
		}
		c.L.Unlock()
	}()

	ctx, span := trace.StartSpan(ctx, "agent_get_resource_token")
	go func() {
		defer span.End()
		defer cancel()
		c.L.Lock()

		isWaiting = true
		for !a.isResourceAvailableLocked(memory, cpuQuota, isAsync) && ctx.Err() == nil {
			c.Wait()
		}
		isWaiting = false

		if ctx.Err() != nil {
			c.L.Unlock()
			return
		}

		t := a.allocResourcesLocked(memory, cpuQuota, isAsync)
		c.L.Unlock()

		select {
		case ch <- t:
		case <-ctx.Done():
			// if we can't send b/c nobody is waiting anymore, need to decrement here
			t.Close()
		}
	}()

	return ch
}

// WaitAsyncResource will send a signal on the returned channel when RAM and CPU in-use
// in the async area is less than high water mark
func (a *resourceTracker) WaitAsyncResource(ctx context.Context) chan struct{} {
	ch := make(chan struct{}, 1)

	isWaiting := false
	c := a.cond

	// if we find a resource token, shut down the thread waiting on ctx finish.
	// alternatively, if the ctx is done, wake up the cond loop.
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()
		c.L.Lock()
		if isWaiting {
			c.Broadcast()
		}
		c.L.Unlock()
	}()

	ctx, span := trace.StartSpan(ctx, "agent_wait_async_resource")
	go func() {
		defer span.End()
		defer cancel()
		c.L.Lock()
		isWaiting = true
		for (a.ramAsyncUsed >= a.ramAsyncHWMark || a.cpuAsyncUsed >= a.cpuAsyncHWMark) && ctx.Err() == nil {
			c.Wait()
		}
		isWaiting = false
		c.L.Unlock()

		if ctx.Err() == nil {
			ch <- struct{}{}
		}
	}()

	return ch
}

func minUint64(a, b uint64) uint64 {
	if a <= b {
		return a
	}
	return b
}

func maxUint64(a, b uint64) uint64 {
	if a >= b {
		return a
	}
	return b
}

func clampUint64(val, min, max uint64) uint64 {
	val = minUint64(val, max)
	val = maxUint64(val, min)
	return val
}

func (a *resourceTracker) initializeCPU(cfg *Config) {

	var maxSyncCPU, maxAsyncCPU, cpuAsyncHWMark uint64

	// Use all available CPU from go.runtime in non-linux systems. We ignore
	// non-linux container implementations and their limits on CPU if there's any.
	// (This is also the default if we cannot determine limits from proc or sysfs)
	totalCPU := uint64(runtime.NumCPU() * 1000)
	availCPU := totalCPU

	if runtime.GOOS == "linux" {

		// Why do we prefer /proc/cpuinfo for Linux and not just use runtime.NumCPU?
		// This is because NumCPU is sched_getaffinity based and we prefer to check
		// cgroup which will more likely be same cgroup for container runtime
		numCPU, err := checkProcCPU()
		if err != nil {
			logrus.WithError(err).Error("Error checking for CPU, falling back to runtime CPU count.")
		} else {
			totalCPU = 1000 * numCPU
			availCPU = totalCPU
		}

		// Clamp further if cgroups CFS quota/period limits are in place
		cgroupCPU := checkCgroupCPU()
		if cgroupCPU > 0 {
			availCPU = minUint64(availCPU, cgroupCPU)
		}

		// TODO: check cgroup cpuset to clamp this further. We might be restricted into
		// a subset of CPUs. (eg. /sys/fs/cgroup/cpuset/cpuset.effective_cpus)

		// TODO: skip CPU headroom for ourselves for now
	}

	// now based on cfg, further clamp on calculated values
	if cfg != nil && cfg.MaxTotalCPU != 0 {
		availCPU = minUint64(cfg.MaxTotalCPU, availCPU)
	}

	logrus.WithFields(logrus.Fields{
		"totalCPU": totalCPU,
		"availCPU": availCPU,
	}).Info("available cpu")

	// %20 of cpu for sync only reserve
	maxSyncCPU = uint64(availCPU * 2 / 10)
	maxAsyncCPU = availCPU - maxSyncCPU
	cpuAsyncHWMark = maxAsyncCPU * 8 / 10

	logrus.WithFields(logrus.Fields{
		"cpuSync":        maxSyncCPU,
		"cpuAsync":       maxAsyncCPU,
		"cpuAsyncHWMark": cpuAsyncHWMark,
	}).Info("sync and async cpu reservations")

	if maxSyncCPU == 0 || maxAsyncCPU == 0 {
		logrus.Fatal("Cannot get the proper CPU information to size server")
	}

	if maxSyncCPU+maxAsyncCPU < 1000 {
		logrus.Warn("Severaly Limited CPU: cpuSync + cpuAsync < 1000m (1 CPU)")
	} else if maxAsyncCPU < 1000 {
		logrus.Warn("Severaly Limited CPU: cpuAsync < 1000m (1 CPU)")
	}

	a.cpuAsyncHWMark = cpuAsyncHWMark
	a.cpuSyncTotal = maxSyncCPU
	a.cpuAsyncTotal = maxAsyncCPU
}

func (a *resourceTracker) initializeMemory(cfg *Config) {

	var maxSyncMemory, maxAsyncMemory, ramAsyncHWMark uint64

	availMemory := uint64(DefaultNonLinuxMemory)

	if runtime.GOOS == "linux" {

		// system wide available memory
		totalMemory, err := checkProcMem()
		if err != nil {
			logrus.WithError(err).Fatal("Cannot get the proper memory information to size server.")
		}

		availMemory = totalMemory

		// cgroup limit restriction on memory usage
		cGroupLimit, err := checkCgroupMem()
		if err != nil {
			logrus.WithError(err).Error("Error checking for cgroup memory limits, falling back to host memory available..")
		} else {
			availMemory = minUint64(cGroupLimit, availMemory)
		}

		// clamp the available memory by head room (for docker, ourselves, other processes)
		headRoom, err := getMemoryHeadRoom(availMemory, cfg)
		if err != nil {
			logrus.WithError(err).Fatal("Out of memory")
		}
		availMemory = availMemory - headRoom

		logrus.WithFields(logrus.Fields{
			"totalMemory": totalMemory,
			"headRoom":    headRoom,
			"cgroupLimit": cGroupLimit,
		}).Info("available memory")
	}

	// now based on cfg, further clamp on calculated values
	if cfg != nil && cfg.MaxTotalMemory != 0 {
		availMemory = minUint64(cfg.MaxTotalMemory, availMemory)
	}

	// %20 of ram for sync only reserve
	maxSyncMemory = uint64(availMemory * 2 / 10)
	maxAsyncMemory = availMemory - maxSyncMemory
	ramAsyncHWMark = maxAsyncMemory * 8 / 10

	// For non-linux OS, we expect these (or their defaults) properly configured from command-line/env
	logrus.WithFields(logrus.Fields{
		"availMemory":    availMemory,
		"ramSync":        maxSyncMemory,
		"ramAsync":       maxAsyncMemory,
		"ramAsyncHWMark": ramAsyncHWMark,
	}).Info("sync and async ram reservations")

	if maxSyncMemory == 0 || maxAsyncMemory == 0 {
		logrus.Fatal("Cannot get the proper memory pool information to size server")
	}

	if maxSyncMemory+maxAsyncMemory < 256*Mem1MB {
		logrus.Warn("Severely Limited memory: ramSync + ramAsync < 256MB")
	} else if maxAsyncMemory < 256*Mem1MB {
		logrus.Warn("Severely Limited memory: ramAsync < 256MB")
	}

	a.ramAsyncHWMark = ramAsyncHWMark
	a.ramSyncTotal = maxSyncMemory
	a.ramAsyncTotal = maxAsyncMemory
}

// headroom estimation in order not to consume entire RAM if possible
func getMemoryHeadRoom(usableMemory uint64, cfg *Config) (uint64, error) {

	// get %10 of the RAM
	headRoom := uint64(usableMemory / 10)

	// TODO: improve this pre-fork calculation, we should fetch/query this
	// instead of estimate below.
	// if pre-fork pool is enabled, add 1 MB per pool-item
	if cfg != nil && cfg.PreForkPoolSize != 0 {
		headRoom += Mem1MB * cfg.PreForkPoolSize
	}

	// TODO: improve these calculations.
	// clamp this with 256MB min -- 5GB max
	maxHeadRoom := uint64(5 * Mem1GB)
	minHeadRoom := uint64(256 * Mem1MB)

	if minHeadRoom >= usableMemory {
		return 0, fmt.Errorf("Not enough memory: %v", usableMemory)
	}

	headRoom = clampUint64(headRoom, minHeadRoom, maxHeadRoom)
	return headRoom, nil
}

func readString(fileName string) (string, error) {
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", err
	}
	value := string(b)
	return strings.TrimSpace(value), nil
}

func checkCgroupMem() (uint64, error) {
	value, err := readString("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(value, 10, 64)
}

func checkCgroupCPU() uint64 {

	periodStr, err := readString("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if err != nil {
		return 0
	}
	quotaStr, err := readString("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	if err != nil {
		return 0
	}

	period, err := strconv.ParseUint(periodStr, 10, 64)
	if err != nil {
		logrus.Warn("Cannot parse CFS period", err)
		return 0
	}

	quota, err := strconv.ParseInt(quotaStr, 10, 64)
	if err != nil {
		logrus.Warn("Cannot parse CFS quota", err)
		return 0
	}

	if quota <= 0 || period <= 0 {
		return 0
	}

	return uint64(quota) * 1000 / period
}

var errCantReadMemInfo = errors.New("Didn't find MemAvailable in /proc/meminfo, kernel is probably < 3.14")

func checkProcMem() (uint64, error) {
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

func checkProcCPU() (uint64, error) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	total := uint64(0)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		b := scanner.Text()

		// processor       : 0
		toks := strings.Fields(b)
		if len(toks) == 3 && toks[0] == "processor" && toks[1] == ":" {
			total += 1
		}
	}

	if total == 0 {
		return 0, errors.New("Could not parse cpuinfo")
	}

	return total, nil
}
