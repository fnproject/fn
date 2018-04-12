package common

import (
	"fmt"
	"math"
	"sync"
)

/*
   WaitGroup is used to manage and wait for a collection of
   sessions. It is similar to sync.WaitGroup, but
   AddSession/CloseGroup session is not only thread
   safe but can be executed in any order unlike sync.WaitGroup.

   Once a shutdown is initiated via CloseGroup(), add/rm
   operations will still function correctly, where
   AddSession would return false. In this state,
   CloseGroup() blocks until sessions get drained
   via remove operations.

   It is an error to call AddSession() with invalid values.
   For example, if current session count is 1, AddSession
   can only add more or subtract 1 from this. Caller needs
   to make sure addition/subtraction math is correct when
   using WaitGroup.

   Example usage:

   group := NewWaitGroup()

   for item := range(items) {
       go func(item string) {
           if !group.AddSession(1) {
               // group may be closing or full
               return
           }
           defer group.AddSession(-1)

           // do stuff
       }(item)
   }

   // close the group and wait for active item.
   group.CloseGroup()
*/

type WaitGroup struct {
	cond     *sync.Cond
	closer   chan struct{}
	isClosed bool
	sessions uint64
}

func NewWaitGroup() *WaitGroup {
	return &WaitGroup{
		cond:   sync.NewCond(new(sync.Mutex)),
		closer: make(chan struct{}),
	}
}

// Closer returns a channel that is closed if
// WaitGroup is in closing state
func (r *WaitGroup) Closer() chan struct{} {
	return r.closer
}

// AddSession manipulates the session counter by
// adding or subtracting the delta value. Incrementing
// the session counter is not possible and will set
// return value to false if a close was initiated.
// It's callers responsibility to make sure addition and
// subtraction math is correct.
func (r *WaitGroup) AddSession(delta int64) bool {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if delta >= 0 {
		// we cannot add if we are being shutdown
		if r.isClosed {
			return false
		}

		incr := uint64(delta)

		// we have maxed out
		if r.sessions == math.MaxUint64-incr {
			return false
		}

		r.sessions += incr
	} else {
		decr := uint64(-delta)

		// illegal operation, it's callers responsibility
		// to make sure subtraction and addition math is correct.
		if r.sessions < decr {
			panic(fmt.Sprintf("common.WaitGroup misuse sum=%d decr=%d isClosed=%v",
				r.sessions, decr, r.isClosed))
		}

		r.sessions -= decr

		// subtractions need to notify CloseGroup
		r.cond.Broadcast()
	}
	return true
}

// CloseGroup initiates a close and blocks until
// session counter becomes zero.
func (r *WaitGroup) CloseGroup() {
	r.cond.L.Lock()

	if !r.isClosed {
		r.isClosed = true
		close(r.closer)
	}

	for r.sessions != 0 {
		r.cond.Wait()
	}

	r.cond.L.Unlock()
}

// CloseGroupNB is non-blocking version of CloseGroup
// which returns a channel that can be waited on.
func (r *WaitGroup) CloseGroupNB() chan struct{} {

	// set to closing state immediately
	r.cond.L.Lock()
	if !r.isClosed {
		r.isClosed = true
		close(r.closer)
	}
	r.cond.L.Unlock()

	closer := make(chan struct{})

	go func() {
		defer close(closer)
		r.CloseGroup()
	}()

	return closer
}
