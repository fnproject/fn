package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func setTrackerTestVals(tr *resourceTracker, vals *trackerVals) {
	tr.cond.L.Lock()

	tr.ramTotal = vals.mt
	tr.ramUsed = vals.mu
	tr.ramAsyncHWMark = vals.mam

	tr.cpuTotal = vals.ct
	tr.cpuUsed = vals.cu
	tr.cpuAsyncHWMark = vals.cam

	tr.cond.L.Unlock()
	tr.cond.Broadcast()
}

func getTrackerTestVals(tr *resourceTracker, vals *trackerVals) {

	tr.cond.L.Lock()

	vals.mt = tr.ramTotal
	vals.mu = tr.ramUsed
	vals.mam = tr.ramAsyncHWMark

	vals.ct = tr.cpuTotal
	vals.cu = tr.cpuUsed
	vals.cam = tr.cpuAsyncHWMark

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
	// set set these to known vals (4GB total: 1GB async hw mark)
	vals.mt = 4 * Mem1GB
	vals.mu = 0
	vals.mam = 1 * Mem1GB

	// let's assume 10 CPUs (6 CPU async hw mark)
	vals.ct = 10000
	vals.cu = 0
	vals.cam = 6000
}

func fetchToken(ch <-chan ResourceToken) (ResourceToken, error) {
	select {
	case tok := <-ch:
		return tok, nil
	case <-time.After(time.Duration(500) * time.Millisecond):
		return nil, errors.New("expected token")
	}
}

func isClosed(ch <-chan ResourceToken) bool {
	select {
	case _, ok := <-ch:
		if !ok {
			return true
		}
	default:
	}
	return false
}

func TestResourceAsyncWait(t *testing.T) {

	var vals trackerVals

	trI := NewResourceTracker(nil)

	tr := trI.(*resourceTracker)

	getTrackerTestVals(tr, &vals)
	if vals.mt <= 0 || vals.mu != 0 || vals.mam <= 0 {
		t.Fatalf("faulty init MEM %#v", vals)
	}
	if vals.ct <= 0 || vals.cu != 0 || vals.cam <= 0 {
		t.Fatalf("faulty init CPU %#v", vals)
	}

	vals.setDefaults()

	// should block & wait
	vals.mu = vals.mam
	setTrackerTestVals(tr, &vals)

	ctx1, cancel1 := context.WithCancel(context.Background())
	ch1 := tr.WaitAsyncResource(ctx1)
	defer cancel1()

	select {
	case <-ch1:
		t.Fatal("high water mark MEM over, should not trigger")
	case <-time.After(time.Duration(500) * time.Millisecond):
	}

	// should not block & wait
	vals.mu = 0
	setTrackerTestVals(tr, &vals)

	select {
	case <-ch1:
	case <-time.After(time.Duration(500) * time.Millisecond):
		t.Fatal("high water mark MEM not over, should trigger")
	}

	// get a new channel to prevent previous test interference
	ctx2, cancel2 := context.WithCancel(context.Background())
	ch2 := tr.WaitAsyncResource(ctx2)
	defer cancel2()

	// should block & wait
	vals.cu = vals.cam
	setTrackerTestVals(tr, &vals)

	select {
	case <-ch2:
		t.Fatal("high water mark CPU over, should not trigger")
	case <-time.After(time.Duration(500) * time.Millisecond):
	}

	// should not block & wait
	vals.cu = 0
	setTrackerTestVals(tr, &vals)

	select {
	case <-ch2:
	case <-time.After(time.Duration(500) * time.Millisecond):
		t.Fatal("high water mark CPU not over, should trigger")
	}
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
	ctx, cancel := context.WithCancel(context.Background())
	ch := trI.GetResourceToken(ctx, 4*1024, 1000, false)
	defer cancel()

	_, err := fetchToken(ch)
	if err == nil {
		t.Fatalf("full system should not hand out token")
	}

	// reset back
	vals.setDefaults()
	setTrackerTestVals(tr, &vals)

	tok, err := fetchToken(ch)
	if err != nil {
		t.Fatalf("empty system should hand out token")
	}

	// ask for another 4GB and 10 CPU
	ctx, cancel = context.WithCancel(context.Background())
	ch = trI.GetResourceToken(ctx, 4*1024, 1000, false)
	defer cancel()

	_, err = fetchToken(ch)
	if err == nil {
		t.Fatalf("full system should not hand out token")
	}

	// close means, giant token resources released
	tok.Close()

	tok, err = fetchToken(ch)
	if err != nil {
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
