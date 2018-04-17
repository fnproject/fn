package common

import (
	"fmt"
	"math"
	"sync"
)

/*
   WaitGroup is used to manage and wait for a collection of
   sessions. It is similar to sync.WaitGroup, but
   AddSession/DoneSession/CloseGroup session is not only thread
   safe but can be executed in any order unlike sync.WaitGroup.

   Once a shutdown is initiated via CloseGroup(), add/done
   operations will still function correctly, where
   AddSession would return false. In this state,
   CloseGroup() blocks until sessions get drained
   via DoneSession() operations.

   It is callers responsibility to make sure AddSessions
   and DoneSessions math adds up to >= 0. In other words,
   calling more DoneSessions() than sum of AddSessions()
   will cause panic.

   Example usage:

   group := NewWaitGroup()

   for item := range(items) {
       go func(item string) {
           if !group.AddSession(1) {
               // group may be closing or full
               return
           }
           defer group.DoneSession()

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
// adding the delta value. Incrementing
// the session counter is not possible and will set
// return value to false if a close was initiated.
func (r *WaitGroup) AddSession(delta uint64) bool {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	// we cannot add if we are being shutdown
	if r.isClosed {
		return false
	}

	// we have maxed out
	if r.sessions == math.MaxUint64-delta {
		return false
	}

	r.sessions += delta
	return true
}

// DoneSession decrements 1 from accumulated
// sessions and wakes up listeners when this reaches
// zero.
func (r *WaitGroup) DoneSession() {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	// illegal operation, it's callers responsibility
	// to make sure subtraction and addition math is correct.
	if r.sessions == 0 {
		panic(fmt.Sprintf("common.WaitGroup misuse isClosed=%v", r.isClosed))
	}

	r.sessions -= 1
	if r.sessions == 0 {
		r.cond.Broadcast()
	}
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
