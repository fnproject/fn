package common

import (
	"testing"
	"time"
)

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
	}
	return false
}

func TestWaitGroupEmpty(t *testing.T) {

	wg := NewWaitGroup()

	if !wg.AddSession(0) {
		t.Fatalf("Add 0 should not fail")
	}

	if isClosed(wg.Closer()) {
		t.Fatalf("Should not be closed yet")
	}

	done := wg.CloseGroupNB()

	// gate-on close
	wg.CloseGroup()

	if !isClosed(wg.Closer()) {
		t.Fatalf("Should be closing state")
	}

	if isClosed(done) {
		t.Fatalf("NB Chan I should be closed")
	}

	done = wg.CloseGroupNB()
	if isClosed(done) {
		t.Fatalf("NB Chan II should be closed")
	}
}

func TestWaitGroupBlast(t *testing.T) {
	wg := NewWaitGroup()
	wg.AddSession(1)

	blastRadius := 500
	for i := 0; i < blastRadius; i++ {
		go func(i int) {
			if !wg.AddSession(1) {
				// this is OK, we are creating a race
				// and some cannot make it after main
				// go-routine issues a CloseGroupNB()
				return
			}

			if isClosed(wg.Closer()) {
				t.Fatalf("Should not be closing state")
			}

			wg.DoneSession()
		}(i)
	}

	if isClosed(wg.Closer()) {
		t.Fatalf("Should not be closing state")
	}

	done := wg.CloseGroupNB()

	if !isClosed(wg.Closer()) {
		t.Fatalf("Should be closing state")
	}

	select {
	case <-done:
		t.Fatalf("NB Chan should not be closed yet, since sum is 1")
	case <-time.After(time.Duration(1) * time.Second):
		wg.DoneSession()
	}

	select {
	case <-done:
	case <-time.After(time.Duration(1) * time.Second):
		t.Fatalf("NB Chan should be closed by now, since sum is 0")
	}
}

func TestWaitGroupSingle(t *testing.T) {

	wg := NewWaitGroup()

	if isClosed(wg.Closer()) {
		t.Fatalf("Should not be closing state yet")
	}

	if !wg.AddSession(1) {
		t.Fatalf("Add 1 should not fail")
	}

	if isClosed(wg.Closer()) {
		t.Fatalf("Should not be closing state yet")
	}

	wg.DoneSession()
	// sum should be zero now.

	if !wg.AddSession(2) {
		t.Fatalf("Add 2 should not fail")
	}

	// sum is 2 now
	// initiate shutdown
	done := wg.CloseGroupNB()

	if isClosed(done) {
		t.Fatalf("NB Chan should not be closed yet, since sum is 2")
	}

	wg.DoneSession()

	if wg.AddSession(1) {
		t.Fatalf("Add 1 should fail (we are shutting down)")
	}
	if !isClosed(wg.Closer()) {
		t.Fatalf("Should be closing state")
	}

	// sum is 1 now

	if isClosed(done) {
		t.Fatalf("NB Chan should not be closed yet, since sum is 1")
	}

	if wg.AddSession(0) {
		t.Fatalf("Add 0 should fail (considered positive number and we are closing)")
	}

	if wg.AddSession(100) {
		t.Fatalf("Add 100 should fail (we are shutting down)")
	}

	if !isClosed(wg.Closer()) {
		t.Fatalf("Should be closing state")
	}

	wg.DoneSession()

	// sum is 0 now
	<-done

	if !isClosed(done) {
		t.Fatalf("NB Chan should be closed, since sum is 0")
	}

	if !isClosed(wg.Closer()) {
		t.Fatalf("Should be closing state")
	}
}
