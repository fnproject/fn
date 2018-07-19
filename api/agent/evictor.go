package agent

import (
	"sync"
)

// Evictor For Agent
// Agent hot containers can register themselves as evictable using
// Register/Unregister calls. If a hot container registers itself,
// a starved request can call PerformEviction() to scan the eligible
// hot containers and if a number of these can be evicted to satisfy
// memory+cpu needs of the starved request, then those hot-containers
// are evicted (which is signalled using their channel.)

type tokenKey struct {
	id     string
	slotId string
	memory uint64
	cpu    uint64
}

type EvictToken struct {
	key tokenKey
	C   chan struct{}
}

type Evictor interface {
	// Create an eviction token to be used in register/unregister functions
	GetEvictor(id, slotId string, mem, cpu uint64) *EvictToken

	// register an eviction token with evictor system
	RegisterEvictor(token *EvictToken)

	// unregister an eviction token from evictor system
	UnregisterEvictor(token *EvictToken)

	// perform eviction to satisfy resource requirements of the call
	// returns true if evictions were performed to satisfy the requirements.
	PerformEviction(slotId string, mem, cpu uint64) bool
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

func (tok *EvictToken) isEligible() bool {
	// if no resource limits are in place, then this
	// function is not eligible.
	if tok.key.memory == 0 && tok.key.cpu == 0 {
		return false
	}
	return true
}

func (e *evictor) GetEvictor(id, slotId string, mem, cpu uint64) *EvictToken {
	key := tokenKey{
		id:     id,
		slotId: slotId,
		memory: mem,
		cpu:    cpu,
	}

	return &EvictToken{
		key: key,
		C:   make(chan struct{}),
	}
}

func (e *evictor) RegisterEvictor(token *EvictToken) {
	if !token.isEligible() || token.isEvicted() {
		return
	}

	e.lock.Lock()

	// be paranoid, do not register if it's already there
	_, ok := e.tokens[token.key.id]
	if !ok {
		e.tokens[token.key.id] = token
		e.slots = append(e.slots, token.key)
	}

	e.lock.Unlock()
}

func (e *evictor) UnregisterEvictor(token *EvictToken) {
	if !token.isEligible() || token.isEvicted() {
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
}

func (e *evictor) PerformEviction(slotId string, mem, cpu uint64) bool {
	// if no resources are defined for this function, then
	// we don't know what to do here. We cannot evict anyone
	// in this case.
	if mem == 0 && cpu == 0 {
		return false
	}

	// Our eviction sum so far
	totalMemory := uint64(0)
	totalCpu := uint64(0)
	isSatisfied := false

	var keys []string
	var chans []chan struct{}

	e.lock.Lock()

	for _, val := range e.slots {
		// lets not evict from our own slot queue
		if slotId == val.slotId {
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

		chans = make([]chan struct{}, 0, len(keys))
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

			chans = append(chans, e.tokens[id].C)
			delete(e.tokens, id)
		}
	}

	e.lock.Unlock()

	for _, ch := range chans {
		close(ch)
	}

	return isSatisfied
}
