package agent

import (
	"context"
	"errors"
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

func (vals *trackerVals) setDefaults() {
	// set set these to known vals (4GB total: 1GB sync, 3 async)
	vals.mst = 1 * Mem1GB
	vals.msu = 0
	vals.mat = 3 * Mem1GB
	vals.mau = 0
	vals.mam = 1 * Mem1GB

	// let's assume 10 CPUs (2 CPU sync, 8 CPU async)
	vals.cst = 2000
	vals.csu = 0
	vals.cat = 8000
	vals.cau = 0
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

	trI := NewResourceTracker()

	tr := trI.(*resourceTracker)

	getTrackerTestVals(tr, &vals)
	if vals.mst <= 0 || vals.msu != 0 || vals.mat <= 0 || vals.mau != 0 || vals.mam <= 0 {
		t.Fatalf("faulty init MEM %#v", vals)
	}
	if vals.cst <= 0 || vals.csu != 0 || vals.cat <= 0 || vals.cau != 0 || vals.cam <= 0 {
		t.Fatalf("faulty init CPU %#v", vals)
	}

	vals.setDefaults()

	// should block & wait
	vals.mau = vals.mam
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
	vals.mau = 0
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
	vals.cau = vals.cam
	setTrackerTestVals(tr, &vals)

	select {
	case <-ch2:
		t.Fatal("high water mark CPU over, should not trigger")
	case <-time.After(time.Duration(500) * time.Millisecond):
	}

	// should not block & wait
	vals.cau = 0
	setTrackerTestVals(tr, &vals)

	select {
	case <-ch2:
	case <-time.After(time.Duration(500) * time.Millisecond):
		t.Fatal("high water mark CPU not over, should trigger")
	}
}

func TestResourceGetSimple(t *testing.T) {

	var vals trackerVals
	trI := NewResourceTracker()
	tr := trI.(*resourceTracker)

	vals.setDefaults()

	// let's make it like CPU and MEM are 100% full
	vals.mau = vals.mat
	vals.cau = vals.cat

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
	if vals.msu != 0 || vals.mau != 0 {
		t.Fatalf("faulty state MEM %#v", vals)
	}
	if vals.csu != 0 || vals.cau != 0 {
		t.Fatalf("faulty state CPU %#v", vals)
	}
}

func TestResourceGetCombo(t *testing.T) {

	var vals trackerVals
	trI := NewResourceTracker()
	tr := trI.(*resourceTracker)

	vals.setDefaults()
	setTrackerTestVals(tr, &vals)

	// impossible request
	ctx, cancel := context.WithCancel(context.Background())
	ch := trI.GetResourceToken(ctx, 20*1024, 20000, false)
	_, err := fetchToken(ch)
	if err == nil {
		t.Fatalf("impossible request should never return (error here)")
	}

	cancel()
	ctx, cancel = context.WithCancel(context.Background())

	// let's use up 2 GB of 3GB async pool
	ch = trI.GetResourceToken(ctx, 2*1024, 10, true)
	tok1, err := fetchToken(ch)
	if err != nil {
		t.Fatalf("empty async system should hand out token1")
	}

	cancel()
	ctx, cancel = context.WithCancel(context.Background())

	// remaining 1 GB async
	ch = trI.GetResourceToken(ctx, 1*1024, 11, true)
	tok2, err := fetchToken(ch)
	if err != nil {
		t.Fatalf("empty async system should hand out token2")
	}

	cancel()
	ctx, cancel = context.WithCancel(context.Background())

	// NOW ASYNC POOL IS FULL
	// SYNC POOL HAS 1GB

	// we no longer can get async token
	ch = trI.GetResourceToken(ctx, 1*1024, 12, true)
	_, err = fetchToken(ch)
	if err == nil {
		t.Fatalf("full async system should not hand out a token")
	}

	cancel()
	ctx, cancel = context.WithCancel(context.Background())

	// but we should get 1GB sync token
	ch = trI.GetResourceToken(ctx, 1*1024, 13, false)
	tok3, err := fetchToken(ch)
	if err != nil {
		t.Fatalf("empty sync system should hand out token3")
	}

	cancel()
	ctx, cancel = context.WithCancel(context.Background())

	// NOW ASYNC AND SYNC POOLS ARE FULL

	// this should fail
	ch = trI.GetResourceToken(ctx, 1*1024, 14, false)
	_, err = fetchToken(ch)
	if err == nil {
		t.Fatalf("full system should not hand out a token")
	}

	cancel()
	ctx, cancel = context.WithCancel(context.Background())

	// now let's free up some async pool, release tok2 (1GB)
	tok2.Close()

	// NOW ASYNC POOL HAS 1GB FREE
	// SYNC POOL IS FULL

	// async pool should provide this
	ch = trI.GetResourceToken(ctx, 1*1024, 15, false)
	tok4, err := fetchToken(ch)
	if err != nil {
		t.Fatalf("async system should hand out token4")
	}

	cancel()
	ctx, cancel = context.WithCancel(context.Background())

	// NOW ASYNC AND SYNC POOLS ARE FULL

	tok4.Close()
	tok3.Close()

	// NOW ASYNC POOL HAS 1GB FREE
	// SYNC POOL HAS 1GB FREE

	// now, we ask for 2GB sync token, it should be provided from both async+sync pools
	ch = trI.GetResourceToken(ctx, 2*1024, 16, false)
	tok5, err := fetchToken(ch)
	if err != nil {
		t.Fatalf("async+sync system should hand out token5")
	}

	cancel()

	// NOW ASYNC AND SYNC POOLS ARE FULL

	tok1.Close()
	tok5.Close()

	// attempt to close tok2 twice.. This should be OK.
	tok2.Close()

	// POOLS should all be empty now
	getTrackerTestVals(tr, &vals)
	if vals.msu != 0 || vals.mau != 0 {
		t.Fatalf("faulty state MEM %#v", vals)
	}
	if vals.csu != 0 || vals.cau != 0 {
		t.Fatalf("faulty state CPU %#v", vals)
	}

}
