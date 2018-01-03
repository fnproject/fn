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

	"github.com/sirupsen/logrus"
)

const (
	Mem1MB = 1024 * 1024
	Mem1GB = 1024 * 1024 * 1024
)

// A simple resource (memory, cpu, disk, etc.) tracker for scheduling.
// TODO: add cpu, disk, network IO for future
type ResourceTracker interface {
	WaitAsyncResource() chan struct{}
	// returns a closed channel if the resource can never me met.
	GetResourceToken(ctx context.Context, memory uint64, cpuQuota uint64, isAsync bool) <-chan ResourceToken
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

func NewResourceTracker() ResourceTracker {

	obj := &resourceTracker{
		cond: sync.NewCond(new(sync.Mutex)),
	}

	obj.initializeMemory()
	obj.initializeCPU()
	return obj
}

type ResourceToken interface {
	// Close must be called by any thread that receives a token.
	io.Closer
}

type resourceToken struct {
	decrement func()
}

func (t *resourceToken) Close() error {
	t.decrement()
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
func (a *resourceTracker) isResourcePossible(memory uint64, cpuQuota uint64, isAsync bool) bool {
	if isAsync {
		return memory <= a.ramAsyncTotal && cpuQuota <= a.cpuAsyncTotal
	} else {
		return memory <= a.ramSyncTotal+a.ramAsyncTotal && cpuQuota <= a.cpuSyncTotal+a.cpuAsyncTotal
	}
}

// the received token should be passed directly to launch (unconditionally), launch
// will close this token (i.e. the receiver should not call Close)
func (a *resourceTracker) GetResourceToken(ctx context.Context, memory uint64, cpuQuota uint64, isAsync bool) <-chan ResourceToken {

	memory = memory * Mem1MB

	c := a.cond
	ch := make(chan ResourceToken)

	if !a.isResourcePossible(memory, cpuQuota, isAsync) {
		close(ch)
		return ch
	}

	go func() {
		c.L.Lock()

		for !a.isResourceAvailableLocked(memory, cpuQuota, isAsync) {
			select {
			case <-ctx.Done():
				c.L.Unlock()
				return
			default:
			}

			c.Wait()
		}

		var asyncMem, syncMem uint64
		var asyncCPU, syncCPU uint64

		if isAsync {
			// async uses async pool only
			asyncMem = memory
			asyncCPU = cpuQuota
		} else if a.ramSyncTotal-a.ramSyncUsed >= memory && a.cpuSyncTotal-a.cpuSyncUsed >= cpuQuota {
			// if sync fits in sync pool
			syncMem = memory
			syncCPU = cpuQuota
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
		c.L.Unlock()

		t := &resourceToken{decrement: func() {
			c.L.Lock()
			a.ramAsyncUsed -= asyncMem
			a.ramSyncUsed -= syncMem
			a.cpuAsyncUsed -= asyncCPU
			a.cpuSyncUsed -= syncCPU
			c.L.Unlock()

			// WARNING: yes, we wake up everyone even async waiters when only sync pool has space, but
			// the cost of this spurious wake up is unlikely to impact much performance. Simpler
			// to use one cond variable for the time being.
			c.Broadcast()
		}}

		select {
		case ch <- t:
		case <-ctx.Done():
			// if we can't send b/c nobody is waiting anymore, need to decrement here
			t.Close()
		}
	}()

	return ch
}

// WaitAsyncResource will send a signal on the returned channel when RAM in-use
// in the async area is less than high water mark
func (a *resourceTracker) WaitAsyncResource() chan struct{} {
	ch := make(chan struct{})

	c := a.cond
	go func() {
		c.L.Lock()
		for a.ramAsyncUsed >= a.ramAsyncHWMark && a.cpuAsyncUsed >= a.cpuAsyncHWMark {
			c.Wait()
		}
		c.L.Unlock()
		ch <- struct{}{}
		// TODO this could leak forever (only in shutdown, blech)
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

func (a *resourceTracker) initializeCPU() {

	var maxSyncCPU, maxAsyncCPU, cpuAsyncHWMark uint64

	cpuShares := uint64(runtime.NumCPU() * 100)

	if runtime.GOOS == "linux" {

		availCPU := cpuShares

		// for linux, let's check if we have a cgroups quota, if yes, then clamp cpuShares
		// with it.
		cgroupCPU := checkCgroupCPU()
		if cgroupCPU > 0 {
			availCPU = minUint64(cpuShares, cgroupCPU)
		}

		// skip CPU headroom for ourselves for now

		logrus.WithFields(logrus.Fields{
			"totalCPU": cpuShares,
			"availCPU": availCPU,
		}).Info("available cpu")

		// %20 of cpu for sync only reserve
		maxSyncCPU = uint64(availCPU * 2 / 10)
		maxAsyncCPU = availCPU - maxSyncCPU
		cpuAsyncHWMark = maxAsyncCPU * 8 / 10
	} else {
		maxSyncCPU = cpuShares / 4
		maxAsyncCPU = cpuShares * 3 / 4
		cpuAsyncHWMark = cpuShares / 2
	}

	logrus.WithFields(logrus.Fields{
		"cpuSync":        maxSyncCPU,
		"cpuAsync":       maxAsyncCPU,
		"cpuAsyncHWMark": cpuAsyncHWMark,
	}).Info("sync and async cpu reservations")

	if maxSyncCPU == 0 || maxAsyncCPU == 0 {
		logrus.Fatal("Cannot get the proper CPU information to size server")
	}

	a.cpuAsyncHWMark = cpuAsyncHWMark
	a.cpuSyncTotal = maxSyncCPU
	a.cpuAsyncTotal = maxAsyncCPU
}

func (a *resourceTracker) initializeMemory() {

	var maxSyncMemory, maxAsyncMemory, ramAsyncHWMark uint64

	if runtime.GOOS == "linux" {

		// system wide available memory
		totalMemory, err := checkProcMem()
		if err != nil {
			logrus.WithError(err).Fatal("Cannot get the proper memory information to size server.")
		}

		availMemory := totalMemory

		// cgroup limit restriction on memory usage
		cGroupLimit, err := checkCgroupMem()
		if err != nil {
			logrus.WithError(err).Error("Error checking for cgroup memory limits, falling back to host memory available..")
		} else {
			availMemory = minUint64(cGroupLimit, availMemory)
		}

		// clamp the available memory by head room (for docker, ourselves, other processes)
		headRoom, err := getMemoryHeadRoom(availMemory)
		if err != nil {
			logrus.WithError(err).Fatal("Out of memory")
		}
		availMemory = availMemory - headRoom

		logrus.WithFields(logrus.Fields{
			"totalMemory": totalMemory,
			"availMemory": availMemory,
			"headRoom":    headRoom,
			"cgroupLimit": cGroupLimit,
		}).Info("available memory")

		// %20 of ram for sync only reserve
		maxSyncMemory = uint64(availMemory * 2 / 10)
		maxAsyncMemory = availMemory - maxSyncMemory
		ramAsyncHWMark = maxAsyncMemory * 8 / 10

	} else {
		// non-linux: assume 512MB sync only memory and 1.5GB async + sync memory
		maxSyncMemory = 512 * Mem1MB
		maxAsyncMemory = (1024 + 512) * Mem1MB
		ramAsyncHWMark = 1024 * Mem1MB
	}

	// For non-linux OS, we expect these (or their defaults) properly configured from command-line/env
	logrus.WithFields(logrus.Fields{
		"ramSync":        maxSyncMemory,
		"ramAsync":       maxAsyncMemory,
		"ramAsyncHWMark": ramAsyncHWMark,
	}).Info("sync and async ram reservations")

	if maxSyncMemory == 0 || maxAsyncMemory == 0 {
		logrus.Fatal("Cannot get the proper memory pool information to size server")
	}

	if maxSyncMemory+maxAsyncMemory < 256*Mem1MB {
		logrus.Warn("Severaly Limited memory: ramSync + ramAsync < 256MB")
	} else if maxAsyncMemory < 256*Mem1MB {
		logrus.Warn("Severaly Limited memory: ramAsync < 256MB")
	}

	a.ramAsyncHWMark = ramAsyncHWMark
	a.ramSyncTotal = maxSyncMemory
	a.ramAsyncTotal = maxAsyncMemory
}

// headroom estimation in order not to consume entire RAM if possible
func getMemoryHeadRoom(usableMemory uint64) (uint64, error) {

	// get %10 of the RAM
	headRoom := uint64(usableMemory / 10)

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
	f, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
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
		return 0
	}

	quota, err := strconv.ParseInt(quotaStr, 10, 64)
	if err != nil {
		return 0
	}

	if quota <= 0 || period <= 0 {
		return 0
	}

	return uint64(quota) * 100 / period
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
