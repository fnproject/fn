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

// A simple resource (memory, cpu, disk, etc.) tracker for scheduling.
// TODO: add cpu, disk, network IO for future
type ResourceTracker interface {
	WaitAsyncResource() chan struct{}
	// returns a closed channel if the resource can never me met.
	GetResourceToken(ctx context.Context, memory uint64, isAsync bool) <-chan ResourceToken
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
}

func NewResourceTracker() ResourceTracker {

	obj := &resourceTracker{
		cond: sync.NewCond(new(sync.Mutex)),
	}

	obj.initializeMemory()
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

func (a *resourceTracker) isResourceAvailableLocked(memory uint64, isAsync bool) bool {

	asyncAvail := a.ramAsyncTotal - a.ramAsyncUsed
	syncAvail := a.ramSyncTotal - a.ramSyncUsed

	// For sync functions, we can steal from async pool. For async, we restrict it to sync pool
	if isAsync {
		return asyncAvail >= memory
	} else {
		return asyncAvail+syncAvail >= memory
	}
}

// is this request possible to meet? If no, fail quick
func (a *resourceTracker) isResourcePossible(memory uint64, isAsync bool) bool {
	if isAsync {
		return memory <= a.ramAsyncTotal
	} else {
		return memory <= a.ramSyncTotal+a.ramAsyncTotal
	}
}

// the received token should be passed directly to launch (unconditionally), launch
// will close this token (i.e. the receiver should not call Close)
func (a *resourceTracker) GetResourceToken(ctx context.Context, memory uint64, isAsync bool) <-chan ResourceToken {

	memory = memory * 1024 * 1024

	c := a.cond
	ch := make(chan ResourceToken)

	if !a.isResourcePossible(memory, isAsync) {
		close(ch)
		return ch
	}

	go func() {
		c.L.Lock()

		for !a.isResourceAvailableLocked(memory, isAsync) {
			select {
			case <-ctx.Done():
				c.L.Unlock()
				return
			default:
			}

			c.Wait()
		}

		var asyncMem, syncMem uint64

		if isAsync {
			// async uses async pool only
			asyncMem = memory
		} else if a.ramSyncTotal-a.ramSyncUsed >= memory {
			// if sync fits in sync pool
			syncMem = memory
		} else {
			// if sync fits async + sync pool
			syncMem = a.ramSyncTotal - a.ramSyncUsed
			asyncMem = memory - syncMem
		}

		a.ramAsyncUsed += asyncMem
		a.ramSyncUsed += syncMem
		c.L.Unlock()

		t := &resourceToken{decrement: func() {
			c.L.Lock()
			a.ramAsyncUsed -= asyncMem
			a.ramSyncUsed -= syncMem
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
		for a.ramSyncUsed >= a.ramAsyncHWMark {
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

func (a *resourceTracker) initializeMemory() {

	var maxSyncMemory, maxAsyncMemory, ramAsyncHWMark uint64

	if runtime.GOOS == "linux" {

		// system wide available memory
		totalMemory, err := checkProc()
		if err != nil {
			logrus.WithError(err).Fatal("Cannot get the proper memory information to size server.")
		}

		availMemory := totalMemory

		// cgroup limit restriction on memory usage
		cGroupLimit, err := checkCgroup()
		if err != nil {
			logrus.WithError(err).Error("Error checking for cgroup memory limits, falling back to host memory available..")
		} else {
			availMemory = minUint64(cGroupLimit, availMemory)
		}

		// clamp the available memory by head room (for docker, ourselves, other processes)
		headRoom := getMemoryHeadRoom(availMemory)
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
		maxSyncMemory = 512 * 1024 * 1024
		maxAsyncMemory = (1024 + 512) * 1024 * 1024
		ramAsyncHWMark = 1024 * 1024 * 1024
	}

	// For non-linux OS, we expect these (or their defaults) properly configured from command-line/env
	logrus.WithFields(logrus.Fields{
		"ramSync":        maxSyncMemory,
		"ramAsync":       maxAsyncMemory,
		"ramAsyncHWMark": ramAsyncHWMark,
	}).Info("sync and async reservations")

	if maxSyncMemory == 0 || maxAsyncMemory == 0 {
		logrus.Fatal("Cannot get the proper memory pool information to size server")
	}

	a.ramAsyncHWMark = ramAsyncHWMark
	a.ramSyncTotal = maxSyncMemory
	a.ramAsyncTotal = maxAsyncMemory
}

// headroom estimation in order not to consume entire RAM if possible
func getMemoryHeadRoom(usableMemory uint64) uint64 {

	// get %10 of the RAM
	headRoom := uint64(usableMemory / 10)

	// clamp this with 256MB min -- 5GB max
	maxHeadRoom := uint64(5 * 1024 * 1024 * 1024)
	minHeadRoom := uint64(256 * 1024 * 1024)
	minHeadRoom = minUint64(minHeadRoom, usableMemory)

	headRoom = clampUint64(headRoom, minHeadRoom, maxHeadRoom)
	return headRoom
}

func checkCgroup() (uint64, error) {
	f, err := os.Open("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return 0, err
	}
	limBytes := string(b)
	limBytes = strings.TrimSpace(limBytes)
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
