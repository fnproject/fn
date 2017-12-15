package agent

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type testSlot struct {
	id       uint64
	err      error
	isClosed bool
}

func (a *testSlot) exec(ctx context.Context, call *call) error {
	return nil
}

func (a *testSlot) Close() error {
	if a.isClosed {
		return fmt.Errorf("id=%d already closed %v", a.id, a)
	}
	a.isClosed = true
	return nil
}

func (a *testSlot) Error() error {
	return a.err
}

func NewTestSlot(id uint64) Slot {
	mySlot := &testSlot{
		id: id,
	}
	return mySlot
}

func TestSlotQueueBasic1(t *testing.T) {

	maxId := uint64(10)
	slotName := "test1"

	slots := make([]Slot, 0, maxId)
	tokens := make([]*slotToken, 0, maxId)

	obj := NewSlotQueue(slotName)

	outChan := obj.getDequeueChan()

	select {
	case z := <-outChan:
		t.Fatalf("Should not get anything from queue: %v", z)
	default:
	}

	// create slots
	for id := uint64(0); id < maxId; id += 1 {
		slots = append(slots, NewTestSlot(id))
	}

	// queue a few slots here
	for id := uint64(0); id < maxId; id += 1 {
		tok := obj.queueSlot(slots[id])

		innerTok := tok.slot.(*testSlot)

		// check for slot id match
		if innerTok != slots[id] {
			t.Fatalf("queued testSlot does not match with slotToken.slot %v vs %v", innerTok, slots[id])
		}

		tokens = append(tokens, tok)
	}

	// Now according to LIFO semantics, we should get 9,8,7,5,4,3,2,1,0 if we dequeued right now.
	// but let's eject 9
	if !obj.ejectSlot(tokens[9]) {
		t.Fatalf("Cannot eject slotToken: %v", tokens[9])
	}
	// let eject 0
	if !obj.ejectSlot(tokens[0]) {
		t.Fatalf("Cannot eject slotToken: %v", tokens[0])
	}
	// let eject 5
	if !obj.ejectSlot(tokens[5]) {
		t.Fatalf("Cannot eject slotToken: %v", tokens[5])
	}
	// try ejecting 5 again, it should fail
	if obj.ejectSlot(tokens[5]) {
		t.Fatalf("Shouldn't be able to eject slotToken: %v", tokens[5])
	}

	// now we should get 8
	select {
	case z := <-outChan:
		if z.id != 8 {
			t.Fatalf("Bad slotToken received: %v", z)
		}

		if !z.acquireSlot() {
			t.Fatalf("Cannot acquire slotToken received: %v", z)
		}

	case <-time.After(time.Duration(1) * time.Second):
		t.Fatal("timeout in waiting slotToken")
	}

	// now we should get 7
	select {
	case z := <-outChan:
		if z.id != 7 {
			t.Fatalf("Bad slotToken received: %v", z)
		}

		// eject it before we can consume
		if !obj.ejectSlot(tokens[7]) {
			t.Fatalf("Cannot eject slotToken: %v", tokens[2])
		}

		// we shouldn't be able to consume an ejected slotToken
		if z.acquireSlot() {
			t.Fatalf("We should not be able to acquire slotToken received: %v", z)
		}

	case <-time.After(time.Duration(1) * time.Second):
		t.Fatal("timeout in waiting slotToken")
	}

	// now we've got: 4,3,2,1
	// let's destroy this queue
	obj.destroySlotQueue()

	// we should get nothing
	select {
	case z, ok := <-outChan:
		if ok {
			t.Fatalf("Should not get anything from queue: %v", z)
		}
	default:
	}

	// attempt to queue again should fail
	tok := obj.queueSlot(slots[0])
	if tok != nil {
		t.Fatalf("Attempt to queue into closed slotQueue should fail")
	}

	// all of our slots should be busy (acquired now)
	for _, v := range tokens {
		if v.isBusy == uint32(0) {
			t.Fatalf("Slot not busy after ejects and destroyed queue %v", v)
		}
	}

	stats1 := obj.getStats()
	isNeeded, stats2 := obj.isNewContainerNeeded()

	if stats1 != stats2 {
		t.Fatalf("Faulty stats %v != %v", stats1, stats2)
	}

	if isNeeded {
		t.Fatalf("Shouldn't need a container for destroy slotQueue")
	}
}
