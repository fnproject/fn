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
// TODO: improve memory implementation
// TODO: add cpu, disk, network IO for future
type ResourceTracker interface {
	WaitAsyncResource() chan struct{}
	GetResourceToken(ctx context.Context, call *call) <-chan ResourceToken
}

type resourceTracker struct {
	// cond protects access to ramUsed
	cond *sync.Cond
	// ramTotal is the total accessible memory by this process
	ramTotal uint64
	// ramUsed is ram reserved for running containers. idle hot containers
	// count against ramUsed.
	ramUsed uint64
}

func NewResourceTracker() ResourceTracker {

	obj := &resourceTracker{
		cond:     sync.NewCond(new(sync.Mutex)),
		ramTotal: getAvailableMemory(),
	}

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

// the received token should be passed directly to launch (unconditionally), launch
// will close this token (i.e. the receiver should not call Close)
func (a *resourceTracker) GetResourceToken(ctx context.Context, call *call) <-chan ResourceToken {

	memory := call.Memory * 1024 * 1024

	c := a.cond
	ch := make(chan ResourceToken)

	go func() {
		c.L.Lock()
		for (a.ramUsed + memory) > a.ramTotal {
			select {
			case <-ctx.Done():
				c.L.Unlock()
				return
			default:
			}

			c.Wait()
		}

		a.ramUsed += memory
		c.L.Unlock()

		t := &resourceToken{decrement: func() {
			c.L.Lock()
			a.ramUsed -= memory
			c.L.Unlock()
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

// GetAsyncResource will send a signal on the returned channel when at least half of
// the available RAM on this machine is free.
func (a *resourceTracker) WaitAsyncResource() chan struct{} {
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

func getAvailableMemory() uint64 {
	const tooBig = 322122547200 // #300GB or 0, biggest aws instance is 244GB

	var availableMemory uint64 = tooBig
	if runtime.GOOS == "linux" {
		var err error
		availableMemory, err = checkCgroup()
		if err != nil {
			logrus.WithError(err).Error("Error checking for cgroup memory limits, falling back to host memory available..")
		}
		if availableMemory >= tooBig || availableMemory <= 0 {
			// Then -m flag probably wasn't set, so use max available on system
			availableMemory, err = checkProc()
			if availableMemory >= tooBig || availableMemory <= 0 {
				logrus.WithError(err).Fatal("Cannot get the proper memory information to size server. You must specify the maximum available memory by passing the -m command with docker run when starting the server via docker, eg:  `docker run -m 2G ...`")
			}
		}
	} else {
		// This still lets 10-20 functions execute concurrently assuming a 2GB machine.
		availableMemory = 2 * 1024 * 1024 * 1024
	}

	logrus.WithFields(logrus.Fields{"ram": availableMemory}).Info("available memory")

	return availableMemory
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
