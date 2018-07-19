package agent

import (
	"testing"
)

func getACall(id, slot string, mem, cpu int) (string, string, uint64, uint64) {
	return id, slot, uint64(mem), uint64(cpu)
}

func TestEvictorSimple01(t *testing.T) {
	evictor := NewEvictor()

	slotId := "slot1"
	id1, _, mem1, cpu1 := getACall("id1", slotId, 1, 100)
	id2, _, mem2, cpu2 := getACall("id2", slotId, 1, 100)

	token1 := evictor.GetEvictor(id1, slotId, mem1, cpu1)
	token2 := evictor.GetEvictor(id2, slotId, mem2, cpu2)

	evictor.RegisterEvictor(token1)
	evictor.RegisterEvictor(token2)

	if evictor.PerformEviction(slotId, mem1, cpu1) {
		t.Fatalf("We should not be able to self evict")
	}
	if evictor.PerformEviction("foo", 0, 0) {
		t.Fatalf("We should not be able to evict: zero cpu/mem")
	}
	if evictor.PerformEviction("foo", 1, 300) {
		t.Fatalf("We should not be able to evict (resource not enough)")
	}

	if token1.isEvicted() {
		t.Fatalf("should not be evicted")
	}
	if token2.isEvicted() {
		t.Fatalf("should not be evicted")
	}

	if !evictor.PerformEviction("foo", 1, 100) {
		t.Fatalf("We should be able to evict")
	}

	if !token1.isEvicted() {
		t.Fatalf("should be evicted")
	}
	if token2.isEvicted() {
		t.Fatalf("should not be evicted")
	}

	evictor.UnregisterEvictor(token1)
	evictor.UnregisterEvictor(token2)
}

func TestEvictorSimple02(t *testing.T) {
	evictor := NewEvictor()

	id1, slotId1, mem1, cpu1 := getACall("id1", "slot1", 1, 100)
	id2, slotId2, mem2, cpu2 := getACall("id2", "slot1", 1, 100)

	token1 := evictor.GetEvictor(id1, slotId1, mem1, cpu1)
	token2 := evictor.GetEvictor(id2, slotId2, mem2, cpu2)

	// add/rm/add
	evictor.RegisterEvictor(token1)
	evictor.UnregisterEvictor(token1)
	evictor.RegisterEvictor(token1)

	// add/rm
	evictor.RegisterEvictor(token2)
	evictor.UnregisterEvictor(token2)

	if evictor.PerformEviction(slotId1, mem1, cpu1) {
		t.Fatalf("We should not be able to self evict")
	}
	if evictor.PerformEviction("foo", 0, 0) {
		t.Fatalf("We should not be able to evict: zero cpu/mem")
	}
	if token1.isEvicted() {
		t.Fatalf("should not be evicted")
	}

	evictor.UnregisterEvictor(token1)

	// not registered... but should be OK
	evictor.UnregisterEvictor(token2)

	if evictor.PerformEviction("foo", mem1, cpu1) {
		t.Fatalf("We should not be able to evict (unregistered)")
	}
	if token1.isEvicted() {
		t.Fatalf("should not be evicted")
	}
	if token2.isEvicted() {
		t.Fatalf("should not be evicted (not registered")
	}
}

func TestEvictorSimple03(t *testing.T) {
	evictor := NewEvictor()

	taboo := "foo"
	slotId := "slot1"
	id0, slotId0, mem0, cpu0 := getACall("id0", taboo, 1, 100)
	id1, _, mem1, cpu1 := getACall("id1", slotId, 1, 100)
	id2, _, mem2, cpu2 := getACall("id2", slotId, 1, 100)
	id3, _, mem3, cpu3 := getACall("id3", slotId, 1, 100)

	token0 := evictor.GetEvictor(id0, slotId0, mem0, cpu0)
	token1 := evictor.GetEvictor(id1, slotId, mem1, cpu1)
	token2 := evictor.GetEvictor(id2, slotId, mem2, cpu2)
	token3 := evictor.GetEvictor(id3, slotId, mem3, cpu3)

	evictor.RegisterEvictor(token0)
	evictor.RegisterEvictor(token1)
	evictor.RegisterEvictor(token2)
	evictor.RegisterEvictor(token3)

	if !evictor.PerformEviction(taboo, 1, 200) {
		t.Fatalf("We should be able to evict")
	}

	// same slot id should not be evicted...
	if token0.isEvicted() {
		t.Fatalf("should not be evicted")
	}
	if !token1.isEvicted() {
		t.Fatalf("should be evicted")
	}
	if !token2.isEvicted() {
		t.Fatalf("should be evicted")
	}
	// two tokens should be enough...
	if token3.isEvicted() {
		t.Fatalf("should not be evicted")
	}

	evictor.UnregisterEvictor(token0)
	evictor.UnregisterEvictor(token1)
	evictor.UnregisterEvictor(token2)
	evictor.UnregisterEvictor(token3)
}
