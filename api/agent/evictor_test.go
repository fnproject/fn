package agent

import (
	"testing"
)

func getACall(slot string, mem, cpu int) (string, uint64, uint64) {
	return slot, uint64(mem), uint64(cpu)
}

func TestEvictorSimple01(t *testing.T) {
	evictor := NewEvictor()

	slotId := "slot1"
	_, mem1, cpu1 := getACall(slotId, 1, 100)
	_, mem2, cpu2 := getACall(slotId, 1, 100)

	token1 := evictor.CreateEvictToken(slotId, mem1, cpu1)
	token2 := evictor.CreateEvictToken(slotId, mem2, cpu2)

	token1.SetEvictable(true)
	token2.SetEvictable(true)

	if len(evictor.PerformEviction(slotId, mem1, cpu1)) > 0 {
		t.Fatalf("We should not be able to self evict")
	}
	if len(evictor.PerformEviction("foo", 0, 0)) > 0 {
		t.Fatalf("We should not be able to evict: zero cpu/mem")
	}
	if len(evictor.PerformEviction("foo", 1, 300)) > 0 {
		t.Fatalf("We should not be able to evict (resource not enough)")
	}

	if token1.isEvicted() {
		t.Fatalf("should not be evicted")
	}
	if token2.isEvicted() {
		t.Fatalf("should not be evicted")
	}

	if len(evictor.PerformEviction("foo", 1, 100)) != 1 {
		t.Fatalf("We should be able to evict")
	}

	if !token1.isEvicted() {
		t.Fatalf("should be evicted")
	}
	if token2.isEvicted() {
		t.Fatalf("should not be evicted")
	}

	evictor.DeleteEvictToken(token1)
	evictor.DeleteEvictToken(token2)
}

func TestEvictorSimple02(t *testing.T) {
	evictor := NewEvictor()

	slotId1, mem1, cpu1 := getACall("slot1", 1, 100)
	slotId2, mem2, cpu2 := getACall("slot1", 1, 100)

	token1 := evictor.CreateEvictToken(slotId1, mem1, cpu1)
	token2 := evictor.CreateEvictToken(slotId2, mem2, cpu2)

	// add/rm/add
	token1.SetEvictable(true)
	token1.SetEvictable(false)
	token1.SetEvictable(true)

	// add/rm
	token2.SetEvictable(true)
	token2.SetEvictable(false)

	if len(evictor.PerformEviction(slotId1, mem1, cpu1)) > 0 {
		t.Fatalf("We should not be able to self evict")
	}
	if len(evictor.PerformEviction("foo", 0, 0)) > 0 {
		t.Fatalf("We should not be able to evict: zero cpu/mem")
	}
	if token1.isEvicted() {
		t.Fatalf("should not be evicted")
	}

	token1.SetEvictable(false)

	// not registered... but should be OK
	token2.SetEvictable(false)

	if len(evictor.PerformEviction("foo", mem1, cpu1)) > 0 {
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
	slotId0, mem0, cpu0 := getACall(taboo, 1, 100)
	_, mem1, cpu1 := getACall(slotId, 1, 100)
	_, mem2, cpu2 := getACall(slotId, 1, 100)
	_, mem3, cpu3 := getACall(slotId, 1, 100)

	token0 := evictor.CreateEvictToken(slotId0, mem0, cpu0)
	token1 := evictor.CreateEvictToken(slotId, mem1, cpu1)
	token2 := evictor.CreateEvictToken(slotId, mem2, cpu2)
	token3 := evictor.CreateEvictToken(slotId, mem3, cpu3)

	token0.SetEvictable(true)
	token1.SetEvictable(true)
	token2.SetEvictable(true)
	token3.SetEvictable(true)

	if len(evictor.PerformEviction(taboo, 1, 200)) == 0 {
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

	evictor.DeleteEvictToken(token0)
	evictor.DeleteEvictToken(token1)
	evictor.DeleteEvictToken(token2)
	evictor.DeleteEvictToken(token3)
}
