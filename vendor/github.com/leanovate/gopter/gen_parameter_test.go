package gopter_test

import (
	"math/rand"
	"testing"

	"github.com/leanovate/gopter"
)

type fixedSeed struct {
	fixed int64
}

func (f *fixedSeed) Int63() int64    { return f.fixed }
func (f *fixedSeed) Seed(seed int64) { f.fixed = seed }

func TestGenParameters(t *testing.T) {
	parameters := &gopter.GenParameters{
		MaxSize: 100,
		Rng:     rand.New(&fixedSeed{}),
	}

	if !parameters.NextBool() {
		t.Error("Bool should be true")
	}
	if parameters.NextInt64() != 0 {
		t.Error("int64 should be 0")
	}
	if parameters.NextUint64() != 0 {
		t.Error("uint64 should be 0")
	}

	parameters.Rng.Seed(1)
	if parameters.NextBool() {
		t.Error("Bool should be false")
	}
	if parameters.NextInt64() != 1 {
		t.Error("int64 should be 1")
	}
	if parameters.NextUint64() != 3 {
		t.Error("uint64 should be 3")
	}

	parameters.Rng.Seed(2)
	if !parameters.NextBool() {
		t.Error("Bool should be true")
	}
	if parameters.NextInt64() != -2 {
		t.Error("int64 should be -2")
	}
	if parameters.NextUint64() != 6 {
		t.Error("uint64 should be 6")
	}

	param1 := parameters.CloneWithSeed(1024)
	param2 := parameters.CloneWithSeed(1024)

	for i := 0; i < 100; i++ {
		if param1.NextInt64() != param2.NextInt64() {
			t.Error("cloned parameters create different random numbers")
		}
	}
}
