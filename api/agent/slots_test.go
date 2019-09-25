package agent

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
)

type testSlot struct {
	id       uint64
	err      error
	isClosed bool
}

func (a *testSlot) exec(ctx context.Context, call *call) error {
	return nil
}

func (a *testSlot) SetError(err error) {
	a.err = err
}

func (a *testSlot) Close() {
	if a.isClosed {
		panic(fmt.Errorf("id=%d already closed %v", a.id, a))
	}
	a.isClosed = true
}

var _ Slot = &testSlot{}

func NewTestSlot(id uint64) Slot {
	mySlot := &testSlot{
		id: id,
	}
	return mySlot
}

func checkGetTokenId(t *testing.T, a *slotQueue, dur time.Duration, id uint64) error {

	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	outChan := a.startDequeuer(ctx)

	for {
		select {
		case z := <-outChan:
			if !a.acquireSlot(z) {
				continue
			}

			z.slot.Close()

			if z.id != id {
				return fmt.Errorf("Bad slotToken received: %#v expected: %d", z, id)
			}
			return nil

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func TestSlotQueueBasic1(t *testing.T) {

	maxId := uint64(10)
	slotName := "test1"

	slots := make([]Slot, 0, maxId)
	tokens := make([]*slotToken, 0, maxId)

	obj := NewSlotQueue(slotName)

	timeout := time.Duration(500) * time.Millisecond
	err := checkGetTokenId(t, obj, timeout, 6)
	if err == nil {
		t.Fatalf("Should not get anything from queue")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf(err.Error())
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
			t.Fatalf("queued testSlot does not match with slotToken.slot %#v vs %#v", innerTok, slots[id])
		}

		tokens = append(tokens, tok)
	}

	// Now according to LIFO semantics, we should get 9,8,7,6,5,4,3,2,1,0 if we dequeued right now.
	// but let's acquire 9
	if !obj.acquireSlot(tokens[9]) {
		t.Fatalf("Cannot acquire slotToken: %#v", tokens[9])
	}
	// let acquire 0
	if !obj.acquireSlot(tokens[0]) {
		t.Fatalf("Cannot acquire slotToken: %#v", tokens[0])
	}
	// let acquire 5
	if !obj.acquireSlot(tokens[5]) {
		t.Fatalf("Cannot acquire slotToken: %#v", tokens[5])
	}
	// try acquire 5 again, it should fail
	if obj.acquireSlot(tokens[5]) {
		t.Fatalf("Shouldn't be able to acquire slotToken: %#v", tokens[5])
	}

	err = checkGetTokenId(t, obj, timeout, 8)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// acquire 7 before we can consume
	if !obj.acquireSlot(tokens[7]) {
		t.Fatalf("Cannot acquire slotToken: %#v", tokens[2])
	}

	err = checkGetTokenId(t, obj, timeout, 6)
	if err != nil {
		t.Fatalf(err.Error())
	}
}

func TestSlotQueueBasic2(t *testing.T) {

	obj := NewSlotQueue("test2")

	if !obj.isIdle() {
		t.Fatalf("Should be idle")
	}

	timeout := time.Duration(500) * time.Millisecond
	err := checkGetTokenId(t, obj, timeout, 6)
	if err == nil {
		t.Fatalf("Should not get anything from queue")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf(err.Error())
	}
}

func statsHelperSet(reqW, reqE, conW, conS, conI, conB uint64) slotQueueStats {
	return slotQueueStats{
		requestStates:   [RequestStateMax]uint64{0, reqW, reqE, 0},
		containerStates: [ContainerStateMax]uint64{0, conW, conS, conI, conB, 0},
	}
}

func TestSlotNewContainerLogic1(t *testing.T) {

	var cur slotQueueStats

	cur = statsHelperSet(0, 0, 0, 0, 0, 0)
	// CASE: There's no queued requests
	if isNewContainerNeeded(&cur) {
		t.Fatalf("Should not need a new container cur: %#v", cur)
	}

	// CASE: There are starters >= queued requests
	cur = statsHelperSet(1, 0, 0, 10, 0, 0)
	if isNewContainerNeeded(&cur) {
		t.Fatalf("Should not need a new container cur: %#v", cur)
	}

	// CASE: There are starters < queued requests
	cur = statsHelperSet(10, 0, 0, 1, 0, 0)
	if !isNewContainerNeeded(&cur) {
		t.Fatalf("Should need a new container cur: %#v", cur)
	}

	// CASE: effective queued requests (idle > requests)
	cur = statsHelperSet(10, 0, 0, 0, 11, 0)
	if isNewContainerNeeded(&cur) {
		t.Fatalf("Should not need a new container cur: %#v", cur)
	}

	// CASE: effective queued requests (idle < requests)
	cur = statsHelperSet(10, 0, 0, 0, 5, 0)
	if !isNewContainerNeeded(&cur) {
		t.Fatalf("Should need a new container cur: %#v", cur)
	}

	// CASE: no executors, but 1 queued request
	cur = statsHelperSet(1, 0, 0, 0, 0, 0)
	if !isNewContainerNeeded(&cur) {
		t.Fatalf("Should need a new container cur: %#v", cur)
	}
}

func TestSlotQueueBasic3(t *testing.T) {

	slotName := "test3"

	obj := NewSlotQueue(slotName)

	slot1 := NewTestSlot(1)
	slot2 := NewTestSlot(2)
	token1 := obj.queueSlot(slot1)
	obj.queueSlot(slot2)

	timeout := time.Duration(500) * time.Millisecond
	err := checkGetTokenId(t, obj, timeout, 1)
	if err != nil {
		t.Fatalf(err.Error())
	}

	// let's acquire 1
	if !obj.acquireSlot(token1) {
		t.Fatalf("should fail to acquire %#v", token1)
	}

	goMax := 10
	out := make(chan error, goMax)
	var wg sync.WaitGroup

	wg.Add(goMax)
	for i := 0; i < goMax; i += 1 {
		go func(id int) {
			defer wg.Done()
			err := checkGetTokenId(t, obj, timeout, 1)
			out <- err
		}(i)
	}

	wg.Wait()

	deadlineErrors := 0
	for i := 0; i < goMax; i += 1 {
		err := <-out
		if err == context.DeadlineExceeded {
			deadlineErrors++
		} else if err == nil {
			t.Fatalf("Unexpected success")
		} else {
			t.Fatalf("Unexpected error: %s", err.Error())
		}
	}

	if deadlineErrors != goMax {
		t.Fatalf("Expected %d got %d deadline exceeded errors", goMax, deadlineErrors)
	}

	err = checkGetTokenId(t, obj, timeout, 2)
	if err != context.DeadlineExceeded {
		t.Fatalf(err.Error())
	}
}

func BenchmarkSlotKey(b *testing.B) {
	appName := "myapp"
	appID := id.New().String()
	fnID := id.New().String()
	image := "fnproject/fn-test-utils"
	const timeout = 1
	const idleTimeout = 20
	const memory = 256
	CPUs := models.MilliCPUs(1000)
	method := "GET"
	url := "http://127.0.0.1:8080/invoke/" + fnID
	payload := "payload"
	typ := "sync"
	cfg := models.Config{
		"FN_APP_NAME": appName,
		"FN_MEMORY":   strconv.Itoa(memory),
		"FN_CPUS":     CPUs.String(),
		"FN_TYPE":     typ,
		"APP_VAR":     "FOO",
	}

	cm := &models.Call{
		Config:      cfg,
		AppID:       appID,
		FnID:        fnID,
		Image:       image,
		Type:        typ,
		Timeout:     timeout,
		IdleTimeout: idleTimeout,
		Memory:      memory,
		CPUs:        CPUs,
		Payload:     payload,
		URL:         url,
		Method:      method,
	}

	call := &call{Call: cm}

	for i := 0; i < b.N; i++ {
		_ = getSlotQueueKey(call, "")
	}
}
