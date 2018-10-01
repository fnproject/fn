package agent

import (
	"sync"
	"sync/atomic"

	"github.com/fnproject/fn/api/id"

	"github.com/sirupsen/logrus"
)

// Evictor For Agent
// Agent hot containers register themselves to the evictor system.
// A starved request can call PerformEviction() to scan the evictable
// hot containers and if a number of these can be evicted to satisfy
// memory+cpu needs of the starved request, then those hot-containers
// are evicted.

type tokenKey struct {
	id     string
	slotId string
	memory uint64
	cpu    uint64
}

type EvictToken struct {
	key       tokenKey
	evictable uint32
	C         chan struct{}
	DoneChan  chan struct{}
}

type Evictor interface {
	// CreateEvictToken creates an eviction token to be used in evictor tracking. Returns
	// an eviction token.
	CreateEvictToken(slotId string, mem, cpu uint64) *EvictToken

	// DeleteEvictToken deletes an eviction token from evictor system
	DeleteEvictToken(token *EvictToken)

	// PerformEviction performs evictions to satisfy cpu & mem arguments
	// and returns a slice of channels for evictions performed. The callers
	// can wait on these channel to ensure evictions are completed.
	PerformEviction(slotId string, mem, cpu uint64) []chan struct{}
}

type evictor struct {
	lock   sync.Mutex
	id     uint64
	tokens map[string]*EvictToken
	slots  []tokenKey
}

func NewEvictor() Evictor {
	return &evictor{
		tokens: make(map[string]*EvictToken),
		slots:  make([]tokenKey, 0),
	}
}

func (tok *EvictToken) isEvicted() bool {
	select {
	case <-tok.C:
		return true
	default:
	}
	return false
}

func (token *EvictToken) SetEvictable(isEvictable bool) {
	val := uint32(0)
	if isEvictable {
		val = 1
	}

	atomic.StoreUint32(&token.evictable, val)
}

func (tok *EvictToken) isEligible() bool {
	// if no resource limits are in place, then this
	// function is not eligible.
	return tok.key.memory != 0 || tok.key.cpu != 0
}

func (e *evictor) CreateEvictToken(slotId string, mem, cpu uint64) *EvictToken {

	key := tokenKey{
		id:     id.New().String(),
		slotId: slotId,
		memory: mem,
		cpu:    cpu,
	}

	token := &EvictToken{
		key:      key,
		C:        make(chan struct{}),
		DoneChan: make(chan struct{}),
	}

	if !token.isEligible() {
		return token
	}

	e.lock.Lock()

	_, ok := e.tokens[token.key.id]
	if ok {
		logrus.Fatalf("id collusion key=%+v", key)
	}

	e.tokens[token.key.id] = token
	e.slots = append(e.slots, token.key)

	e.lock.Unlock()

	return token
}

func (e *evictor) DeleteEvictToken(token *EvictToken) {
	if !token.isEligible() {
		return
	}

	e.lock.Lock()

	for idx, val := range e.slots {
		if val.id == token.key.id {
			e.slots = append(e.slots[:idx], e.slots[idx+1:]...)
			break
		}
	}
	delete(e.tokens, token.key.id)

	e.lock.Unlock()

	close(token.DoneChan)
}

func (e *evictor) PerformEviction(slotId string, mem, cpu uint64) []chan struct{} {
	var notifyChans []chan struct{}

	// if no resources are defined for this function, then
	// we don't know what to do here. We cannot evict anyone
	// in this case.
	if mem == 0 && cpu == 0 {
		return notifyChans
	}

	// Our eviction sum so far
	totalMemory := uint64(0)
	totalCpu := uint64(0)
	isSatisfied := false

	var keys []string
	var completionChans []chan struct{}

	e.lock.Lock()

	for _, val := range e.slots {
		// lets not evict from our own slot queue
		if slotId == val.slotId {
			continue
		}
		// descend into map to verify evictable state
		if atomic.LoadUint32(&e.tokens[val.id].evictable) == 0 {
			continue
		}

		totalMemory += val.memory
		totalCpu += val.cpu
		keys = append(keys, val.id)

		// did we satisfy the need?
		if totalMemory >= mem && totalCpu >= cpu {
			isSatisfied = true
			break
		}
	}

	// If we can satisfy the need, then let's commit/perform eviction
	if isSatisfied {

		notifyChans = make([]chan struct{}, 0, len(keys))
		completionChans = make([]chan struct{}, 0, len(keys))

		idx := 0
		for _, id := range keys {

			// do not initialize idx, we continue where we left off
			// since keys are in order from above.
			for ; idx < len(e.slots); idx++ {
				if id == e.slots[idx].id {
					e.slots = append(e.slots[:idx], e.slots[idx+1:]...)
					break
				}
			}

			notifyChans = append(notifyChans, e.tokens[id].C)
			completionChans = append(completionChans, e.tokens[id].DoneChan)

			delete(e.tokens, id)
		}
	}

	e.lock.Unlock()

	for _, ch := range notifyChans {
		close(ch)
	}

	return completionChans
}
