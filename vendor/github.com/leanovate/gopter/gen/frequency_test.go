package gen_test

import (
	"math/rand"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

type fixedSeed struct {
	fixed int64
}

func (f fixedSeed) Int63() int64    { return f.fixed }
func (f fixedSeed) Seed(seed int64) {}

func fixedParameters(size int, fixed int64) *gopter.GenParameters {
	return &gopter.GenParameters{
		MaxSize: size,
		Rng: rand.New(fixedSeed{
			fixed: fixed,
		}),
	}
}

func TestFrequency(t *testing.T) {
	zeroNine := gen.Frequency(map[int]gopter.Gen{
		0: gen.Const("zero"),
		9: gen.Const("nine"),
	})
	value, ok := zeroNine(fixedParameters(10, 0)).Retrieve()
	if !ok {
		t.FailNow()
	}
	if value.(string) != "zero" {
		t.Errorf("Invalid value for 0: %#v", value)
	}
	for i := int64(1); i < int64(10); i++ {
		value, ok = zeroNine(fixedParameters(10, i<<32)).Retrieve()
		if !ok {
			t.FailNow()
		}
		if value.(string) != "nine" {
			t.Errorf("Invalid value for %d: %#v", i, value)
		}
	}
}
