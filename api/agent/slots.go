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

type Slot interface {
	exec(ctx context.Context, call *call) error
	Close() error
	Error() error
}

// slotQueueMgr manages hot container slotQueues
type slotQueueMgr struct {
	hMu sync.Mutex // protects hot
	hot map[string]*slotQueue
}

type SlotQueueMetricType int

const (
	SlotQueueRunner  SlotQueueMetricType = iota // container is running
	SlotQueueStarter                            // container is launching
	SlotQueueWaiter                             // requests are waiting
	SlotQueueIdle                               // hot container is running, but idle (free tokens)
	SlotQueueLast
)

// counters per state and moving avg of time spent in each state
type slotQueueStats struct {
	states    [SlotQueueLast]uint64
	latencies [SlotQueueLast]uint64
}

type slotToken struct {
	slot    Slot
	trigger chan struct{}
	id      uint64
	isBusy  uint32
}

// LIFO queue that exposes input/output channels along
// with runner/waiter tracking for agent
type slotQueue struct {
	key       string
	cond      *sync.Cond
	slots     []*slotToken
	nextId    uint64
	signaller chan bool
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
		key:       key,
		cond:      sync.NewCond(new(sync.Mutex)),
		slots:     make([]*slotToken, 0),
		signaller: make(chan bool, 1),
	}

	return obj
}

func (a *slotToken) acquireSlot() bool {
	// let's get the lock
	if !atomic.CompareAndSwapUint32(&a.isBusy, 0, 1) {
		return false
	}

	// now we have the lock, push the trigger
	close(a.trigger)
	return true
}

func (a *slotQueue) ejectSlot(s *slotToken) bool {
	// let's get the lock
	if !atomic.CompareAndSwapUint32(&s.isBusy, 0, 1) {
		return false
	}

	a.cond.L.Lock()
	for i := 0; i < len(a.slots); i++ {
		if a.slots[i].id == s.id {
			a.slots = append(a.slots[:i], a.slots[i+1:]...)
			break
		}
	}
	a.cond.L.Unlock()

	s.slot.Close()
	// now we have the lock, push the trigger
	close(s.trigger)
	return true
}

func (a *slotQueue) startDequeuer(ctx context.Context) chan *slotToken {

	isWaiting := false
	output := make(chan *slotToken)

	go func() {
		<-ctx.Done()
		a.cond.L.Lock()
		if isWaiting {
			a.cond.Broadcast()
		}
		a.cond.L.Unlock()
	}()

	go func() {
		for {
			a.cond.L.Lock()

			isWaiting = true
			for len(a.slots) <= 0 && (ctx.Err() == nil) {
				a.cond.Wait()
			}
			isWaiting = false

			if ctx.Err() != nil {
				a.cond.L.Unlock()
				return
			}

			// pop
			item := a.slots[len(a.slots)-1]
			a.slots = a.slots[:len(a.slots)-1]
			a.cond.L.Unlock()

			select {
			case output <- item: // good case (dequeued)
			case <-item.trigger: // ejected (eject handles cleanup)
			case <-ctx.Done(): // time out or cancel from caller
				// consume slot, we let the hot container queue the slot again
				if item.acquireSlot() {
					item.slot.Close()
				}
			}
		}
	}()

	return output
}

func (a *slotQueue) queueSlot(slot Slot) *slotToken {

	token := &slotToken{slot, make(chan struct{}), 0, 0}

	a.cond.L.Lock()
	token.id = a.nextId
	a.slots = append(a.slots, token)
	a.nextId += 1
	a.cond.L.Unlock()

	a.cond.Broadcast()
	return token
}

// isIdle() returns true is there's no activity for this slot queue. This
// means no one is waiting, running or starting.
func (a *slotQueue) isIdle() bool {
	var partySize uint64

	a.statsLock.Lock()
	partySize = a.stats.states[SlotQueueWaiter] + a.stats.states[SlotQueueStarter] + a.stats.states[SlotQueueRunner]
	a.statsLock.Unlock()

	return partySize == 0
}

func (a *slotQueue) getStats() slotQueueStats {
	var out slotQueueStats
	a.statsLock.Lock()
	out = a.stats
	a.statsLock.Unlock()
	return out
}

func isNewContainerNeeded(cur *slotQueueStats) bool {

	idlers := cur.states[SlotQueueIdle]
	starters := cur.states[SlotQueueStarter]
	waiters := cur.states[SlotQueueWaiter]

	// we expect idle containers to immediately pick up
	// any waiters. We assume non-idle containers busy.
	effectiveWaiters := uint64(0)
	if idlers < waiters {
		effectiveWaiters = waiters - idlers
	}

	if effectiveWaiters == 0 {
		return false
	}

	// if containers are starting, do not start more than effective waiters
	if starters > 0 && starters >= effectiveWaiters {
		return false
	}

	return true
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
	a.stats.latencies[metricIdx] = (a.stats.latencies[metricIdx]*5 + latency*5) / 10
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
func (a *slotQueueMgr) getSlotQueue(call *call) (*slotQueue, bool) {

	key := getSlotQueueKey(call)

	a.hMu.Lock()
	slots, ok := a.hot[key]
	if !ok {
		slots = NewSlotQueue(key)
		a.hot[key] = slots
	}
	slots.enterState(SlotQueueWaiter)
	a.hMu.Unlock()

	return slots, !ok
}

// currently unused. But at some point, we need to age/delete old
// slotQueues.
func (a *slotQueueMgr) deleteSlotQueue(slots *slotQueue) bool {
	isDeleted := false

	a.hMu.Lock()
	if slots.isIdle() {
		delete(a.hot, slots.key)
		isDeleted = true
	}
	a.hMu.Unlock()

	return isDeleted
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
	fmt.Fprint(hash, call.CPUs, "\x00")
	fmt.Fprint(hash, call.Format, "\x00")

	// we have to sort these before printing, yay. TODO do better
	keys := make([]string, 0, len(call.Config))
	for k := range call.Config {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprint(hash, k, "\x00", call.Config[k], "\x00")
	}

	var buf [sha1.Size]byte
	return string(hash.Sum(buf[:0]))
}
