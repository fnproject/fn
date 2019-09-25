package agent

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"
)

//
// slotQueueMgr keeps track of hot container slotQueues where each slotQueue
// provides for multiple consumers/producers. slotQueue also stores
// a few basic stats in slotStats.
//

type Slot interface {
	exec(ctx context.Context, call *call) error
	SetError(err error)
	Close()
}

// slotQueueMgr manages hot container slotQueues
type slotQueueMgr struct {
	hMu sync.Mutex // protects hot
	hot map[string]*slotQueue
}

// request and container states
type slotQueueStats struct {
	requestStates   [RequestStateMax]uint64
	containerStates [ContainerStateMax]uint64
}

type slotToken struct {
	slot    Slot
	trigger chan struct{}
	id      uint64
	isBusy  uint32
}

type slotCaller struct {
	id     string
	notify chan error      // notification to caller
	done   <-chan struct{} // caller done
}

// LIFO queue that exposes input/output channels along
// with runner/waiter tracking for agent
type slotQueue struct {
	key       string
	cond      *sync.Cond
	slots     []*slotToken
	nextId    uint64
	signaller chan *slotCaller
	statsLock sync.Mutex // protects stats below
	stats     slotQueueStats

	authLock  sync.Mutex
	authToken string
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
		signaller: make(chan *slotCaller, 1),
	}

	return obj
}

func (a *slotQueue) acquireSlot(s *slotToken) bool {
	// let's get the lock
	if !atomic.CompareAndSwapUint32(&s.isBusy, 0, 1) {
		return false
	}

	a.cond.L.Lock()
	// common case: acquired slots are usually at the end
	for i := len(a.slots) - 1; i >= 0; i-- {
		if a.slots[i].id == s.id {
			a.slots = append(a.slots[:i], a.slots[i+1:]...)
			break
		}
	}
	a.cond.L.Unlock()

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

			item := a.slots[len(a.slots)-1]
			a.cond.L.Unlock()

			select {
			case output <- item: // good case (dequeued)
			case <-item.trigger: // ejected (eject handles cleanup)
			case <-ctx.Done(): // time out or cancel from caller
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
	var isIdle bool

	a.statsLock.Lock()

	isIdle = a.stats.requestStates[RequestStateWait] == 0 &&
		a.stats.requestStates[RequestStateExec] == 0 &&
		a.stats.containerStates[ContainerStateWait] == 0 &&
		a.stats.containerStates[ContainerStateStart] == 0 &&
		a.stats.containerStates[ContainerStateIdle] == 0 &&
		a.stats.containerStates[ContainerStatePaused] == 0 &&
		a.stats.containerStates[ContainerStateBusy] == 0

	a.statsLock.Unlock()

	return isIdle
}

func (a *slotQueue) getStats() slotQueueStats {
	var out slotQueueStats
	a.statsLock.Lock()
	out = a.stats
	a.statsLock.Unlock()
	return out
}

func isNewContainerNeeded(cur *slotQueueStats) bool {

	idleWorkers := cur.containerStates[ContainerStateIdle] + cur.containerStates[ContainerStatePaused]
	starters := cur.containerStates[ContainerStateStart]
	startWaiters := cur.containerStates[ContainerStateWait]

	queuedRequests := cur.requestStates[RequestStateWait]

	// we expect idle containers to immediately pick up
	// any waiters. We assume non-idle containers busy.
	effectiveWaiters := uint64(0)
	if idleWorkers < queuedRequests {
		effectiveWaiters = queuedRequests - idleWorkers
	}

	if effectiveWaiters == 0 {
		return false
	}

	// we expect resource waiters to eventually transition
	// into starters.
	effectiveStarters := starters + startWaiters

	// if containers are starting, do not start more than effective waiters
	if effectiveStarters > 0 && effectiveStarters >= effectiveWaiters {
		return false
	}

	return true
}

func (a *slotQueue) enterRequestState(reqType RequestStateType) {
	if reqType > RequestStateNone && reqType < RequestStateMax {
		a.statsLock.Lock()
		a.stats.requestStates[reqType] += 1
		a.statsLock.Unlock()
	}
}

func (a *slotQueue) exitRequestState(reqType RequestStateType) {
	if reqType > RequestStateNone && reqType < RequestStateMax {
		a.statsLock.Lock()
		a.stats.requestStates[reqType] -= 1
		a.statsLock.Unlock()
	}
}

func (a *slotQueue) enterContainerState(conType ContainerStateType) {
	if conType > ContainerStateNone && conType < ContainerStateMax {
		a.statsLock.Lock()
		a.stats.containerStates[conType] += 1
		a.statsLock.Unlock()
	}
}

func (a *slotQueue) exitContainerState(conType ContainerStateType) {
	if conType > ContainerStateNone && conType < ContainerStateMax {
		a.statsLock.Lock()
		a.stats.containerStates[conType] -= 1
		a.statsLock.Unlock()
	}
}

func (a *slotQueue) setAuthToken(val string) {
	a.authLock.Lock()
	a.authToken = val
	a.authLock.Unlock()
}

func (a *slotQueue) getAuthToken() string {
	var val string
	a.authLock.Lock()
	val = a.authToken
	a.authLock.Unlock()
	return val
}

// getSlot must ensure that if it receives a slot, it will be returned, otherwise
// a container will be locked up forever waiting for slot to free.
func (a *slotQueueMgr) getSlotQueue(key string) (*slotQueue, bool) {

	a.hMu.Lock()
	slots, ok := a.hot[key]
	if !ok {
		slots = NewSlotQueue(key)
		a.hot[key] = slots
	}
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

var shapool = &sync.Pool{New: func() interface{} { return sha256.New() }}

// TODO do better; once we have app+fn versions this function
// can be simply app+fn ids & version
func getSlotQueueKey(call *call, slotExtns string) string {
	// return a sha256 hash of a (hopefully) unique string of all the config
	// values, to make map lookups quicker [than the giant unique string]

	hash := shapool.Get().(hash.Hash)
	hash.Reset()
	defer shapool.Put(hash)

	hash.Write(unsafeBytes(call.AppID))
	hash.Write(unsafeBytes("\x00"))
	hash.Write(unsafeBytes(call.SyslogURL))
	hash.Write(unsafeBytes("\x00"))
	hash.Write(unsafeBytes(call.FnID))
	hash.Write(unsafeBytes("\x00"))
	hash.Write(unsafeBytes(call.Image))
	hash.Write(unsafeBytes("\x00"))

	// these are all static in size we only need to delimit the whole block of them
	var byt [8]byte
	binary.LittleEndian.PutUint32(byt[:4], uint32(call.Timeout))
	hash.Write(byt[:4])

	binary.LittleEndian.PutUint32(byt[:4], uint32(call.IdleTimeout))
	hash.Write(byt[:4])

	binary.LittleEndian.PutUint32(byt[:4], uint32(call.TmpFsSize))
	hash.Write(byt[:4])

	binary.LittleEndian.PutUint64(byt[:], call.Memory)
	hash.Write(byt[:])

	binary.LittleEndian.PutUint64(byt[:], uint64(call.CPUs))
	hash.Write(byt[:])
	hash.Write(unsafeBytes("\x00"))

	// we have to sort these before printing, yay.
	// TODO if we had a max size for config const we could avoid this!
	keys := make([]string, 0, len(call.Config))
	for k := range call.Config {
		i := sort.SearchStrings(keys, k)
		keys = append(keys, "")
		copy(keys[i+1:], keys[i:])
		keys[i] = k
	}

	for _, k := range keys {
		hash.Write(unsafeBytes(k))
		hash.Write(unsafeBytes("\x00"))
		hash.Write(unsafeBytes(call.Config[k]))
		hash.Write(unsafeBytes("\x00"))
	}

	// we need to additionally delimit config and annotations to eliminate overlap bug
	hash.Write(unsafeBytes("\x00"))

	keys = keys[:0] // clear keys
	for k := range call.Annotations {
		i := sort.SearchStrings(keys, k)
		keys = append(keys, "")
		copy(keys[i+1:], keys[i:])
		keys[i] = k
	}

	for _, k := range keys {
		hash.Write(unsafeBytes(k))
		hash.Write(unsafeBytes("\x00"))
		v, _ := call.Annotations.Get(k)
		hash.Write(v)
		hash.Write(unsafeBytes("\x00"))
	}

	if slotExtns != "" {
		hash.Write(unsafeBytes(slotExtns))
		hash.Write(unsafeBytes("\x00"))
	}

	var buf [sha256.Size]byte
	hash.Sum(buf[:0])
	return string(buf[:])
}

// WARN: this is read only
func unsafeBytes(a string) []byte {
	strHeader := (*reflect.StringHeader)(unsafe.Pointer(&a))

	var b []byte
	byteHeader := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	byteHeader.Data = strHeader.Data

	// need to take the length of `a` here to ensure it's alive until after we update b's Data
	// field since the garbage collector can collect a variable once it is no longer used
	// not when it goes out of scope, for more details see https://github.com/golang/go/issues/9046
	l := len(a)
	byteHeader.Len = l
	byteHeader.Cap = l
	return b
}
