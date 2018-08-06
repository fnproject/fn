package gen_test

import (
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestWeighted(t *testing.T) {
	weighted := gen.Weighted([]gen.WeightedGen{
		{Weight: 1, Gen: gen.Const("A")},
		{Weight: 2, Gen: gen.Const("B")},
		{Weight: 7, Gen: gen.Const("C")},
	})
	results := make(map[string]int)
	for i := int64(0); i < int64(1000); i++ {
		result, ok := weighted.Sample()
		if !ok {
			t.FailNow()
		}
		results[result.(string)]++
	}
	expectedResults := map[string]int{
		"A": 100,
		"B": 200,
		"C": 700,
	}
	delta := 50
	for _, value := range []string{"A", "B", "C"} {
		result := results[value]
		expected := expectedResults[value]
		if result < expected-delta || result > expected+delta {
			t.Errorf(
				"Result %d for %v falls outside acceptable range %d, %d",
				result,
				value,
				expected-delta,
				expected+delta)
		}
	}
}
