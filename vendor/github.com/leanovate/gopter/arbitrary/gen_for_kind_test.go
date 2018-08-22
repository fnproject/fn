package arbitrary_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/arbitrary"
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

func TestArbitrariesSimple(t *testing.T) {
	arbitraries := arbitrary.DefaultArbitraries()

	gen := arbitraries.GenForType(reflect.TypeOf(true))
	commonGeneratorTest(t, "bool", gen, func(value interface{}) bool {
		_, ok := value.(bool)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(0))
	commonGeneratorTest(t, "int", gen, func(value interface{}) bool {
		_, ok := value.(int)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(uint(0)))
	commonGeneratorTest(t, "uint", gen, func(value interface{}) bool {
		_, ok := value.(uint)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(int8(0)))
	commonGeneratorTest(t, "int8", gen, func(value interface{}) bool {
		_, ok := value.(int8)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(uint8(0)))
	commonGeneratorTest(t, "uint8", gen, func(value interface{}) bool {
		_, ok := value.(uint8)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(int16(0)))
	commonGeneratorTest(t, "int16", gen, func(value interface{}) bool {
		_, ok := value.(int16)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(uint16(0)))
	commonGeneratorTest(t, "uint16", gen, func(value interface{}) bool {
		_, ok := value.(uint16)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(int32(0)))
	commonGeneratorTest(t, "int32", gen, func(value interface{}) bool {
		_, ok := value.(int32)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(uint32(0)))
	commonGeneratorTest(t, "uint32", gen, func(value interface{}) bool {
		_, ok := value.(uint32)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(int64(0)))
	commonGeneratorTest(t, "int64", gen, func(value interface{}) bool {
		_, ok := value.(int64)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(uint64(0)))
	commonGeneratorTest(t, "uint64", gen, func(value interface{}) bool {
		_, ok := value.(uint64)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(float32(0)))
	commonGeneratorTest(t, "float32", gen, func(value interface{}) bool {
		_, ok := value.(float32)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(float64(0)))
	commonGeneratorTest(t, "float64", gen, func(value interface{}) bool {
		_, ok := value.(float64)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(complex128(0)))
	commonGeneratorTest(t, "complex128", gen, func(value interface{}) bool {
		_, ok := value.(complex128)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(complex64(0)))
	commonGeneratorTest(t, "complex64", gen, func(value interface{}) bool {
		_, ok := value.(complex64)
		return ok
	})

	gen = arbitraries.GenForType(reflect.TypeOf(""))
	commonGeneratorTest(t, "string", gen, func(value interface{}) bool {
		_, ok := value.(string)
		return ok
	})
}
