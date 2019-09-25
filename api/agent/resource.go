package agent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

const (
	Mem1MB = 1024 * 1024
	Mem1GB = 1024 * 1024 * 1024

	// Assume 2GB RAM on non-linux systems
	DefaultNonLinuxMemory = 2048 * Mem1MB
)

var CapacityFull = errors.New("max capacity reached")

type ResourceUtilization struct {
	// CPU in use
	CpuUsed models.MilliCPUs
	// CPU available
	CpuAvail models.MilliCPUs
	// Memory in use in bytes
	MemUsed uint64
	// Memory available in bytes
	MemAvail uint64
}

// A simple resource (memory, cpu, disk, etc.) tracker for scheduling.
// TODO: disk, network IO for future
type ResourceTracker interface {
	// GetResourceToken returns a resource token.
	// Memory is expected to be provided in MB units.
	GetResourceToken(ctx context.Context, memory uint64, cpuQuota models.MilliCPUs) ResourceToken

	// GetResourceTokenNB is the non-blocking equivalent of GetResourceToken. The return value is the
	// resource token itself. If the request cannot be satisfied, a token with CapacityFull error set is
	// returned.
	// Memory is expected to be provided in MB units.
	GetResourceTokenNB(ctx context.Context, memory uint64, cpuQuota models.MilliCPUs) ResourceToken

	// IsResourcePossible returns whether it's possible to fulfill the requested resources on this machine.
	// Memory is expected to be provided in MB units.
	IsResourcePossible(memory uint64, cpuQuota models.MilliCPUs) bool

	// Retrieve current stats/usage
	GetUtilization() ResourceUtilization
}

type resourceTracker struct {
	// cond protects access to ram variables below
	cond *sync.Cond
	// ramTotal is the total usable memory for functions
	ramTotal uint64
	// ramUsed is ram reserved for running containers including hot/idle
	ramUsed uint64
	// cpuTotal is the total usable cpu for functions
	cpuTotal uint64
	// cpuUsed is cpu reserved for running containers including hot/idle
	cpuUsed uint64
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
	NeededCapacity() (uint64, models.MilliCPUs)
}

type resourceToken struct {
	once      sync.Once
	err       error
	needCpu   models.MilliCPUs
	needMem   uint64
	decrement func()
}

func (t *resourceToken) Error() error {
	return t.err
}

func (t *resourceToken) NeededCapacity() (uint64, models.MilliCPUs) {
	return t.needMem, t.needCpu
}

func (t *resourceToken) Close() error {
	t.once.Do(func() {
		if t.decrement != nil {
			t.decrement()
		}
	})
	return nil
}

func (a *resourceTracker) isResourceAvailableLocked(memory uint64, cpuQuota models.MilliCPUs) bool {

	availMem := a.ramTotal - a.ramUsed
	availCPU := a.cpuTotal - a.cpuUsed

	return availMem >= memory && availCPU >= uint64(cpuQuota)
}

func (a *resourceTracker) GetUtilization() ResourceUtilization {
	var util ResourceUtilization

	a.cond.L.Lock()

	util.CpuUsed = models.MilliCPUs(a.cpuUsed)
	util.MemUsed = a.ramUsed

	a.cond.L.Unlock()

	util.CpuAvail = models.MilliCPUs(a.cpuTotal) - util.CpuUsed
	util.MemAvail = a.ramTotal - util.MemUsed

	return util
}

// is this request possible to meet? If no, fail quick
func (a *resourceTracker) IsResourcePossible(memory uint64, cpuQuota models.MilliCPUs) bool {
	memory = memory * Mem1MB
	return memory <= a.ramTotal && uint64(cpuQuota) <= a.cpuTotal
}

func (a *resourceTracker) allocResourcesLocked(memory uint64, cpuQuota models.MilliCPUs) ResourceToken {

	a.ramUsed += memory
	a.cpuUsed += uint64(cpuQuota)

	return &resourceToken{decrement: func() {

		a.cond.L.Lock()
		a.ramUsed -= memory
		a.cpuUsed -= uint64(cpuQuota)
		a.cond.L.Unlock()

		// WARNING: yes, we wake up everyone pool has space, but the cost of this
		// spurious wake up is unlikely to impact much performance. Simpler to use
		// one cond variable for the time being.
		a.cond.Broadcast()
	}}
}

func (a *resourceTracker) GetResourceTokenNB(ctx context.Context, memory uint64, cpuQuota models.MilliCPUs) ResourceToken {

	ctx, span := trace.StartSpan(ctx, "agent_get_resource_token_nb")
	defer span.End()

	if !a.IsResourcePossible(memory, cpuQuota) {
		return &resourceToken{err: CapacityFull, needCpu: cpuQuota, needMem: memory}
	}
	memory = memory * Mem1MB

	var t ResourceToken
	var needMem uint64
	var needCpu models.MilliCPUs

	a.cond.L.Lock()

	availMem := a.ramTotal - a.ramUsed
	availCPU := a.cpuTotal - a.cpuUsed

	if availMem >= memory && availCPU >= uint64(cpuQuota) {
		t = a.allocResourcesLocked(memory, cpuQuota)
	} else {
		if availMem < memory {
			needMem = (memory - availMem) / Mem1MB
		}
		if availCPU < uint64(cpuQuota) {
			needCpu = models.MilliCPUs(uint64(cpuQuota) - availCPU)
		}

		t = &resourceToken{err: CapacityFull, needCpu: needCpu, needMem: needMem}
	}

	a.cond.L.Unlock()
	return t
}

func (a *resourceTracker) GetResourceToken(ctx context.Context, memory uint64, cpuQuota models.MilliCPUs) ResourceToken {

	ctx, span := trace.StartSpan(ctx, "agent_get_resource_token")
	defer span.End()

	var t ResourceToken

	if !a.IsResourcePossible(memory, cpuQuota) || ctx.Err() != nil {
		return t
	}

	c := a.cond
	isWaiting := false

	memory = memory * Mem1MB

	// if we find a resource token, shut down the thread waiting on ctx finish.
	// alternatively, if the ctx is done, wake up the cond loop.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		c.L.Lock()
		if isWaiting {
			c.Broadcast()
		}
		c.L.Unlock()
	}()

	c.L.Lock()

	isWaiting = true
	for !a.isResourceAvailableLocked(memory, cpuQuota) && ctx.Err() == nil {
		c.Wait()
	}
	isWaiting = false

	if ctx.Err() == nil {
		t = a.allocResourcesLocked(memory, cpuQuota)
	}

	c.L.Unlock()
	return t
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
		"total_cpu": totalCPU,
		"avail_cpu": availCPU,
	}).Info("available cpu")

	a.cpuTotal = availCPU

	logrus.WithFields(logrus.Fields{
		"cpu": a.cpuTotal,
	}).Info("cpu reservations")

	if a.cpuTotal == 0 {
		logrus.Fatal("Cannot get the proper CPU information to size server")
	}

	if a.cpuTotal < 1000 {
		logrus.Warn("Severaly Limited CPU: cpu < 1000m (1 CPU)")
	}
}

func (a *resourceTracker) initializeMemory(cfg *Config) {

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
			"total_memory": totalMemory,
			"head_room":    headRoom,
			"cgroup_limit": cGroupLimit,
		}).Info("available memory")
	}

	// now based on cfg, further clamp on calculated values
	if cfg != nil && cfg.MaxTotalMemory != 0 {
		availMemory = minUint64(cfg.MaxTotalMemory, availMemory)
	}

	a.ramTotal = availMemory

	// For non-linux OS, we expect these (or their defaults) properly configured from command-line/env
	logrus.WithFields(logrus.Fields{
		"avail_memory": a.ramTotal,
	}).Info("ram reservations")

	if a.ramTotal == 0 {
		logrus.Fatal("Cannot get the proper memory pool information to size server")
	}

	if a.ramTotal < 256*Mem1MB {
		logrus.Warn("Severely Limited memory: ram < 256MB")
	}
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
	b, err := ioutil.ReadFile(filepath.Clean(fileName))
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
