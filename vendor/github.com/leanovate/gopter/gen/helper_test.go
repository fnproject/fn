package gen_test

import (
	"testing"

	"github.com/leanovate/gopter"
)

func commonGeneratorTest(t *testing.T, name string, gen gopter.Gen, valueCheck func(interface{}) bool) {
	for i := 0; i < 100; i++ {
		value, ok := gen.Sample()

		if !ok || value == nil {
			t.Errorf("Invalid generator result (%s): %#v", name, value)
		} else if !valueCheck(value) {
			t.Errorf("Invalid value (%s): %#v", name, value)
		}

		genResult := gen(gopter.DefaultGenParameters())
		if genResult.Shrinker != nil {
			value, ok := genResult.Retrieve()
			if !ok || value == nil {
				t.Errorf("Invalid generator result (%s): %#v", name, value)
			} else {
				shrink := genResult.Shrinker(value).Filter(genResult.Sieve)
				shrunkValue, ok := shrink()
				if ok && !valueCheck(shrunkValue) {
					t.Errorf("Invalid shrunk value (%s): %#v -> %#v", name, value, shrunkValue)
				}
			}
		}
	}
}
