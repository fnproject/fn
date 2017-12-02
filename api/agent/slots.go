package agent

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sort"
	"sync"
)

const ColdKey = "cold"

type slotMgr struct {
	hMu sync.RWMutex // protects hot
	hot map[string]*slots
}

type slots struct {
	key        string
	isHot      bool
	cond       *sync.Cond // protects slots and isClosed
	slots      []Slot
	input      chan Slot
	output     chan Slot
	isClosed   bool
	runnerLock sync.Mutex // protects runners
	runners    uint64
}

func NewSlotMgr() *slotMgr {
	obj := &slotMgr{
		hot: make(map[string]*slots),
	}
	return obj
}

func NewSlots(key string, isHot bool) *slots {
	obj := &slots{
		key:    key,
		isHot:  isHot,
		cond:   sync.NewCond(new(sync.Mutex)),
		slots:  make([]Slot, 0),
		input:  make(chan Slot),
		output: make(chan Slot),
	}

	// queue
	go func() {
		for {
			select {
			case item, ok := <-obj.input:
				obj.cond.L.Lock()
				if !ok {
					obj.isClosed = true
				} else {
					obj.slots = append(obj.slots, item)
				}
				obj.cond.L.Unlock()
				obj.cond.Broadcast()
			}
		}
	}()

	// dequeue
	go func() {
		for {
			var item Slot
			isClosed := false

			obj.cond.L.Lock()
			for len(obj.slots) <= 0 && !obj.isClosed {
				obj.cond.Wait()
			}
			isClosed = obj.isClosed

			if !isClosed {
				item = obj.slots[len(obj.slots)-1]
				obj.slots = obj.slots[:len(obj.slots)-1]
			}

			obj.cond.L.Unlock()

			if isClosed {
				close(obj.output)
			} else {
				obj.output <- item
			}
		}
	}()

	return obj
}

type Slot interface {
	exec(ctx context.Context, call *call) error
	Close() error
}

func (a *slots) getQueueChan() chan Slot {
	return a.input
}
func (a *slots) getDequeueChan() chan Slot {
	return a.output
}
func (a *slots) enterHotRunner() {
	if !a.isHot {
		return
	}
	a.runnerLock.Lock()
	a.runners += 1
	a.runnerLock.Unlock()
}
func (a *slots) exitHotRunner() {
	if !a.isHot {
		return
	}
	a.runnerLock.Lock()
	if a.runners > 0 {
		a.runners -= 1
	}
	a.runnerLock.Unlock()
}
func (a *slots) getHotRunnerCount() uint64 {
	if !a.isHot {
		return 0
	}
	var runners uint64
	a.runnerLock.Lock()
	runners = a.runners
	a.runnerLock.Unlock()
	return runners
}

// getSlot must ensure that if it receives a slot, it will be returned, otherwise
// a container will be locked up forever waiting for slot to free.
func (a *slotMgr) getSlot(call *call, isHot bool) *slots {
	if !isHot {
		return NewSlots(ColdKey, false)
	}

	key := hotKey(call)

	a.hMu.RLock()
	slots, ok := a.hot[key]
	a.hMu.RUnlock()
	if !ok {
		a.hMu.Lock()
		slots, ok = a.hot[key]
		if !ok {
			slots = NewSlots(key, true)
			a.hot[key] = slots
		}
		a.hMu.Unlock()
	}
	return slots
}

func hotKey(call *call) string {
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
