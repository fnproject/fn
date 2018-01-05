package agent

import (
	"testing"
	"time"
)

func setTrackerTestVals(tr *resourceTracker, vals *trackerVals) {
	tr.cond.L.Lock()

	tr.ramSyncTotal = vals.mst
	tr.ramSyncUsed = vals.msu
	tr.ramAsyncTotal = vals.mat
	tr.ramAsyncUsed = vals.mau
	tr.ramAsyncHWMark = vals.mam

	tr.cpuSyncTotal = vals.cst
	tr.cpuSyncUsed = vals.csu
	tr.cpuAsyncTotal = vals.cat
	tr.cpuAsyncUsed = vals.cau
	tr.cpuAsyncHWMark = vals.cam

	tr.cond.L.Unlock()
	tr.cond.Broadcast()
}

func getTrackerTestVals(tr *resourceTracker, vals *trackerVals) {

	tr.cond.L.Lock()

	vals.mst = tr.ramSyncTotal
	vals.msu = tr.ramSyncUsed
	vals.mat = tr.ramAsyncTotal
	vals.mau = tr.ramAsyncUsed
	vals.mam = tr.ramAsyncHWMark

	vals.cst = tr.cpuSyncTotal
	vals.csu = tr.cpuSyncUsed
	vals.cat = tr.cpuAsyncTotal
	vals.cau = tr.cpuAsyncUsed
	vals.cam = tr.cpuAsyncHWMark

	tr.cond.L.Unlock()
}

// helper to debug print (fields correspond to resourceTracker CPU/MEM fields)
type trackerVals struct {
	mst uint64
	msu uint64
	mat uint64
	mau uint64
	mam uint64
	cst uint64
	csu uint64
	cat uint64
	cau uint64
	cam uint64
}

func TestResourceAsyncMem(t *testing.T) {

	var vals trackerVals

	trI := NewResourceTracker()

	tr := trI.(*resourceTracker)

	getTrackerTestVals(tr, &vals)
	if vals.mst <= 0 || vals.msu != 0 || vals.mat <= 0 || vals.mau != 0 || vals.mam <= 0 {
		t.Fatalf("faulty init %#v", vals)
	}

	// set set these to known vals
	vals.mst = 1 * 1024 * 1024
	vals.msu = 0
	vals.mat = 2 * 1024 * 1024
	vals.mau = 0
	vals.mam = 1 * 1024 * 1024

	// should block & wait
	vals.mau = vals.mam
	setTrackerTestVals(tr, &vals)
	ch := tr.WaitAsyncResource()

	select {
	case <-ch:
		t.Fatal("high water mark over, should not trigger")
	case <-time.After(time.Duration(500) * time.Millisecond):
	}

	// should not block & wait
	vals.mau = 0
	setTrackerTestVals(tr, &vals)

	select {
	case <-ch:
	case <-time.After(time.Duration(500) * time.Millisecond):
		t.Fatal("high water mark not over, should trigger")
	}

}
