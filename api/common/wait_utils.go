package common

import (
	"math"
	"sync"
)

/*
   WaitGroup is used to manage and wait for a collection of
   sessions. It is similar to sync.WaitGroup, but
   AddSession/RmSession/WaitClose session is not only thread
   safe but can be executed in any order unlike sync.WaitGroup.

   Once a shutdown is initiated via CloseGroup(), add/rm
   operations will still function correctly, where
   AddSession would return false error.
   In this state, CloseGroup() blocks until sessions get drained
   via RmSession() calls.

   It is an error to call RmSession without a corresponding
   successful AddSession.

   Example usage:

   group := NewWaitGroup()

   for item := range(items) {
       go func(item string) {
           if !group.AddSession() {
               // group may be closing or full
               return
           }
           defer group.RmSession()

           // do stuff
       }(item)
   }

   // close the group and wait for active item.
   group.CloseGroup()
*/

type WaitGroup struct {
	cond     *sync.Cond
	isClosed bool
	sessions uint64
}

func NewWaitGroup() *WaitGroup {
	return &WaitGroup{
		cond: sync.NewCond(new(sync.Mutex)),
	}
}

func (r *WaitGroup) AddSession() bool {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if r.isClosed {
		return false
	}
	if r.sessions == math.MaxUint64 {
		return false
	}

	r.sessions++
	return true
}

func (r *WaitGroup) RmSession() {
	r.cond.L.Lock()

	if r.sessions == 0 {
		panic("WaitGroup misuse: no sessions to remove")
	}

	r.sessions--
	r.cond.Broadcast()

	r.cond.L.Unlock()
}

func (r *WaitGroup) CloseGroup() {
	r.cond.L.Lock()

	r.isClosed = true
	for r.sessions > 0 {
		r.cond.Wait()
	}

	r.cond.L.Unlock()
}

func (r *WaitGroup) CloseGroupNB() chan struct{} {

	closer := make(chan struct{})

	go func() {
		defer close(closer)
		r.CloseGroup()
	}()

	return closer
}
