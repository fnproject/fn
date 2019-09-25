package agent

import (
	"context"
	"testing"
	"time"
)

func setTrackerTestVals(tr *resourceTracker, vals *trackerVals) {
	tr.cond.L.Lock()

	tr.ramTotal = vals.mt
	tr.ramUsed = vals.mu

	tr.cpuTotal = vals.ct
	tr.cpuUsed = vals.cu

	tr.cond.L.Unlock()
	tr.cond.Broadcast()
}

func getTrackerTestVals(tr *resourceTracker, vals *trackerVals) {

	tr.cond.L.Lock()

	vals.mt = tr.ramTotal
	vals.mu = tr.ramUsed

	vals.ct = tr.cpuTotal
	vals.cu = tr.cpuUsed

	tr.cond.L.Unlock()
}

// helper to debug print (fields correspond to resourceTracker CPU/MEM fields)
type trackerVals struct {
	mt  uint64
	mu  uint64
	mam uint64
	ct  uint64
	cu  uint64
	cam uint64
}

func (vals *trackerVals) setDefaults() {
	// set set these to known vals (4GB total)
	vals.mt = 4 * Mem1GB
	vals.mu = 0
	vals.mam = 1 * Mem1GB

	// let's assume 10 CPUs
	vals.ct = 10000
	vals.cu = 0
	vals.cam = 6000
}

func TestResourceGetSimple(t *testing.T) {

	var vals trackerVals
	trI := NewResourceTracker(nil)
	tr := trI.(*resourceTracker)

	vals.setDefaults()

	// let's make it like CPU and MEM are 100% full
	vals.mu = vals.mt
	vals.cu = vals.ct

	setTrackerTestVals(tr, &vals)

	// ask for 4GB and 10 CPU
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(500)*time.Millisecond)
	tok := trI.GetResourceToken(ctx, 4*1024, 1000)
	defer cancel()

	if tok != nil {
		t.Fatalf("full system should not hand out token")
	}

	// reset back
	vals.setDefaults()
	setTrackerTestVals(tr, &vals)

	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(500)*time.Millisecond)
	tok = trI.GetResourceToken(ctx, 4*1024, 1000)
	defer cancel()
	if tok == nil {
		t.Fatalf("full system should hand out token")
	}

	// ask for another 4GB and 10 CPU
	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(500)*time.Millisecond)
	tok1 := trI.GetResourceToken(ctx, 4*1024, 1000)
	defer cancel()

	if tok1 != nil {
		t.Fatalf("full system should not hand out token")
	}

	// close means, giant token resources released
	tok.Close()

	ctx, cancel = context.WithTimeout(context.Background(), time.Duration(500)*time.Millisecond)
	tok = trI.GetResourceToken(ctx, 4*1024, 1000)
	defer cancel()
	if tok == nil {
		t.Fatalf("full system should hand out token")
	}

	tok.Close()

	// POOLS should all be empty now
	getTrackerTestVals(tr, &vals)
	if vals.mu != 0 {
		t.Fatalf("faulty state MEM %#v", vals)
	}
	if vals.cu != 0 {
		t.Fatalf("faulty state CPU %#v", vals)
	}
}

func TestResourceGetSimpleNB(t *testing.T) {

	var vals trackerVals
	trI := NewResourceTracker(nil)
	tr := trI.(*resourceTracker)

	vals.setDefaults()

	// let's make it like CPU and MEM are 100% full
	vals.mu = vals.mt
	vals.cu = vals.ct

	setTrackerTestVals(tr, &vals)

	// ask for 4GB and 10 CPU
	ctx, cancel := context.WithCancel(context.Background())
	tok := trI.GetResourceTokenNB(ctx, 4*1024, 1000)
	defer cancel()

	if tok.Error() == nil {
		t.Fatalf("full system should not hand out token")
	}

	// reset back
	vals.setDefaults()
	setTrackerTestVals(tr, &vals)

	tok1 := trI.GetResourceTokenNB(ctx, 4*1024, 1000)
	if tok1.Error() != nil {
		t.Fatalf("empty system should hand out token")
	}

	// ask for another 4GB and 10 CPU
	ctx, cancel = context.WithCancel(context.Background())
	tok = trI.GetResourceTokenNB(ctx, 4*1024, 1000)
	defer cancel()

	if tok.Error() == nil {
		t.Fatalf("full system should not hand out token")
	}

	// close means, giant token resources released
	tok1.Close()

	tok = trI.GetResourceTokenNB(ctx, 4*1024, 1000)
	if tok.Error() != nil {
		t.Fatalf("empty system should hand out token")
	}

	tok.Close()

	// POOLS should all be empty now
	getTrackerTestVals(tr, &vals)
	if vals.mu != 0 {
		t.Fatalf("faulty state MEM %#v", vals)
	}
	if vals.cu != 0 {
		t.Fatalf("faulty state CPU %#v", vals)
	}
}
