package agent

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

//
// slotQueueMgr keeps track of hot container slotQueues where each slotQueue
// provides for multiple consumers/producers. slotQueue also stores
// a few basic stats in slotStats.
//

const ColdSlotQueueKey = "cold"

type Slot interface {
	exec(ctx context.Context, call *call) error
	Close() error
	Error() error
}

// slotQueueMgr manages hot container slotQueues
type slotQueueMgr struct {
	hMu sync.RWMutex // protects hot
	hot map[string]*slotQueue
}

type SlotQueueMetricType int

const (
	SlotQueueRunner SlotQueueMetricType = iota
	SlotQueueStarter
	SlotQueueWaiter
	SlotQueueLast
)

// counters per state and moving avg of time spent in each state
type slotQueueStats struct {
	states       [SlotQueueLast]uint64
	latencyCount [SlotQueueLast]uint64
	latencies    [SlotQueueLast]uint64
}

type slotToken struct {
	slot    Slot
	trigger chan struct{}
	isBusy  uint32
}

// LIFO queue that exposes input/output channels along
// with runner/waiter tracking for agent
type slotQueue struct {
	key       string
	cond      *sync.Cond
	slots     []*slotToken
	output    chan *slotToken
	isClosed  bool
	statsLock sync.Mutex // protects stats below
	stats     slotQueueStats
}

func NewSlotQueueMgr() *slotQueueMgr {
	obj := &slotQueueMgr{
		hot: make(map[string]*slotQueue),
	}
	return obj
}

func NewSlotQueue(key string) *slotQueue {
	obj := &slotQueue{
		key:    key,
		cond:   sync.NewCond(new(sync.Mutex)),
		slots:  make([]*slotToken, 0),
		output: make(chan *slotToken),
	}

	go func() {
		for {
			obj.cond.L.Lock()
			for len(obj.slots) <= 0 && !obj.isClosed {
				obj.cond.Wait()
			}
			if obj.isClosed {
				obj.cond.L.Unlock()
				break
			}

			// pop
			item := obj.slots[len(obj.slots)-1]
			obj.slots = obj.slots[:len(obj.slots)-1]
			obj.cond.L.Unlock()

			obj.output <- item
		}
		close(obj.output)

		for _, val := range obj.slots {
			val.slot.Close()
		}

		obj.slots = obj.slots[:0]
	}()

	// TODO: runner go routine to track waiters/runners/starters
	// and spin up new ones

	return obj
}

func (a *slotToken) acquireSignal() bool {
	// let's get the lock
	if !atomic.CompareAndSwapUint32(&a.isBusy, 0, 1) {
		return false
	}

	// now we have the lock, push the trigger
	close(a.trigger)
	return true
}

func (a *slotQueue) destroySlotQueue() {
	doSignal := false
	a.cond.L.Lock()
	if !a.isClosed {
		a.isClosed = true
		doSignal = true
	}
	a.cond.L.Unlock()
	if doSignal {
		a.cond.Signal()
	}
}

func (a *slotQueue) getDequeueChan() chan *slotToken {
	return a.output
}

func (a *slotQueue) queueSlot(slot Slot) *slotToken {

	token := &slotToken{slot, make(chan struct{}), 0}
	isClosed := false

	a.cond.L.Lock()
	if !a.isClosed {
		a.slots = append(a.slots, token)
	} else {
		isClosed = true
	}
	a.cond.L.Unlock()

	if !isClosed {
		a.cond.Signal()
		return token
	}

	return nil
}

func (a *slotQueue) getStats() slotQueueStats {
	var out slotQueueStats
	a.statsLock.Lock()
	out = a.stats
	a.statsLock.Unlock()
	return out
}

func (a *slotQueue) isNewContainerNeeded() (bool, slotQueueStats) {

	stats := a.getStats()

	waiters := stats.states[SlotQueueWaiter]
	if waiters == 0 {
		return false, stats
	}

	// while a container is starting, do not start more than waiters
	starters := stats.states[SlotQueueStarter]
	if starters >= waiters {
		return false, stats
	}

	// no executors? Start a container now.
	executors := starters + stats.states[SlotQueueRunner]
	if executors == 0 {
		return true, stats
	}

	runLat := stats.latencies[SlotQueueRunner]
	waitLat := stats.latencies[SlotQueueWaiter]
	startLat := stats.latencies[SlotQueueStarter]

	// no wait latency? No need to spin up new container
	if waitLat == 0 {
		return false, stats
	}

	// this determines the aggresiveness of the container launch.
	if runLat/executors*2 < waitLat {
		return true, stats
	}
	if runLat < waitLat {
		return true, stats
	}
	if startLat < waitLat {
		return true, stats
	}

	return false, stats
}

func (a *slotQueue) enterState(metricIdx SlotQueueMetricType) {
	a.statsLock.Lock()
	a.stats.states[metricIdx] += 1
	a.statsLock.Unlock()
}

func (a *slotQueue) exitState(metricIdx SlotQueueMetricType) {
	a.statsLock.Lock()
	if a.stats.states[metricIdx] == 0 {
		panic(fmt.Sprintf("BUG: metric tracking fault idx=%v", metricIdx))
	}
	a.stats.states[metricIdx] -= 1
	a.statsLock.Unlock()
}

func (a *slotQueue) recordLatencyLocked(metricIdx SlotQueueMetricType, latency uint64) {
	// exponentially weighted moving average with smoothing factor of 0.5
	// 0.5 is a high value to age older observations fast while filtering
	// some noise. For our purposes, newer observations are much more important
	// than older, but we still would like to low pass some noise.
	// first samples are ignored.
	if a.stats.latencyCount[metricIdx] != 0 {
		a.stats.latencies[metricIdx] = (a.stats.latencies[metricIdx]*5 + latency*5) / 10
	}
	a.stats.latencyCount[metricIdx] += 1
	if a.stats.latencyCount[metricIdx] == 0 {
		a.stats.latencyCount[metricIdx] += 1
	}
}

func (a *slotQueue) recordLatency(metricIdx SlotQueueMetricType, latency uint64) {
	a.statsLock.Lock()
	a.recordLatencyLocked(metricIdx, latency)
	a.statsLock.Unlock()
}

func (a *slotQueue) exitStateWithLatency(metricIdx SlotQueueMetricType, latency uint64) {
	a.statsLock.Lock()
	if a.stats.states[metricIdx] == 0 {
		panic(fmt.Sprintf("BUG: metric tracking fault idx=%v", metricIdx))
	}
	a.stats.states[metricIdx] -= 1
	a.recordLatencyLocked(metricIdx, latency)
	a.statsLock.Unlock()
}

// getSlot must ensure that if it receives a slot, it will be returned, otherwise
// a container will be locked up forever waiting for slot to free.
func (a *slotQueueMgr) getHotSlotQueue(call *call) *slotQueue {

	key := getSlotQueueKey(call)

	a.hMu.RLock()
	slots, ok := a.hot[key]
	a.hMu.RUnlock()
	if !ok {
		a.hMu.Lock()
		slots, ok = a.hot[key]
		if !ok {
			slots = NewSlotQueue(key)
			a.hot[key] = slots
		}
		a.hMu.Unlock()
	}
	return slots
}

// currently unused. But at some point, we need to age/delete old
// slotQueues.
func (a *slotQueueMgr) destroySlotQueue(slots *slotQueue) {
	slots.destroySlotQueue()
	a.hMu.Lock()
	delete(a.hot, slots.key)
	a.hMu.Unlock()
}

func getSlotQueueKey(call *call) string {
	// return a sha1 hash of a (hopefully) unique string of all the config
	// values, to make map lookups quicker [than the giant unique string]

	hash := sha1.New()
	fmt.Fprint(hash, call.AppName, "\x00")
	fmt.Fprint(hash, call.Path, "\x00")
	fmt.Fprint(hash, call.Image, "\x00")
	fmt.Fprint(hash, call.Timeout, "\x00")
	fmt.Fprint(hash, call.IdleTimeout, "\x00")
	fmt.Fprint(hash, call.Memory, "\x00")
	fmt.Fprint(hash, call.Format, "\x00")

	// we have to sort these before printing, yay. TODO do better
	keys := make([]string, 0, len(call.BaseEnv))
	for k := range call.BaseEnv {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprint(hash, k, "\x00", call.BaseEnv[k], "\x00")
	}

	var buf [sha1.Size]byte
	return string(hash.Sum(buf[:0]))
}
