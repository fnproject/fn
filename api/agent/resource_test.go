package agent

import (
	"testing"
	"time"
)

func setTrackerTestVals(tr *resourceTracker, vals *trackerVals) {
	tr.cond.L.Lock()

	tr.ramSyncTotal = vals.st
	tr.ramSyncUsed = vals.su
	tr.ramAsyncTotal = vals.at
	tr.ramAsyncUsed = vals.au
	tr.ramAsyncHWMark = vals.am

	tr.cond.L.Unlock()
	tr.cond.Broadcast()
}

func getTrackerTestVals(tr *resourceTracker, vals *trackerVals) {

	tr.cond.L.Lock()

	vals.st = tr.ramSyncTotal
	vals.su = tr.ramSyncUsed
	vals.at = tr.ramAsyncTotal
	vals.au = tr.ramAsyncUsed
	vals.am = tr.ramAsyncHWMark

	tr.cond.L.Unlock()
}

type trackerVals struct {
	st uint64
	su uint64
	at uint64
	au uint64
	am uint64
}

func TestResourceAsyncMem(t *testing.T) {

	var vals trackerVals

	trI := NewResourceTracker()

	tr := trI.(*resourceTracker)

	getTrackerTestVals(tr, &vals)
	if vals.st <= 0 || vals.su != 0 || vals.at <= 0 || vals.au != 0 || vals.am <= 0 {
		t.Fatalf("faulty init %#v", vals)
	}

	// set set these to known vals
	vals.st = 1 * 1024 * 1024
	vals.su = 0
	vals.at = 2 * 1024 * 1024
	vals.au = 0
	vals.am = 1 * 1024 * 1024

	// should block & wait
	vals.au = vals.am
	setTrackerTestVals(tr, &vals)
	ch := tr.WaitAsyncResource()

	select {
	case <-ch:
		t.Fatal("high water mark over, should not trigger")
	case <-time.After(time.Duration(500) * time.Millisecond):
	}

	// should not block & wait
	vals.au = 0
	setTrackerTestVals(tr, &vals)

	select {
	case <-ch:
	case <-time.After(time.Duration(500) * time.Millisecond):
		t.Fatal("high water mark not over, should trigger")
	}

}
