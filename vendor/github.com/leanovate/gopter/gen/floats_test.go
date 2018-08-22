package gen_test

import (
	"math"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestFloat64(t *testing.T) {
	commonGeneratorTest(t, "float 64", gen.Float64(), func(value interface{}) bool {
		v, ok := value.(float64)
		return ok && !math.IsNaN(v) && !math.IsInf(v, 0)
	})
}

func TestFloat64Range(t *testing.T) {
	fail := gen.Float64Range(200, 100)

	if value, ok := fail.Sample(); value != nil || ok {
		t.Fail()
	}

	commonGeneratorTest(t, "float 64 range", gen.Float64Range(-1234.5, 56789.123), func(value interface{}) bool {
		v, ok := value.(float64)
		return ok && !math.IsNaN(v) && !math.IsInf(v, 0) && v >= -1234.5 && v <= 56789.123
	})
}

func TestFloat32(t *testing.T) {
	commonGeneratorTest(t, "float 32", gen.Float32(), func(value interface{}) bool {
		_, ok := value.(float32)
		return ok
	})
}

func TestFloat32Range(t *testing.T) {
	fail := gen.Float32Range(200, 100)

	if value, ok := fail.Sample(); value != nil || ok {
		t.Fail()
	}

	commonGeneratorTest(t, "float 32 range", gen.Float32Range(-1234.5, 56789.123), func(value interface{}) bool {
		v, ok := value.(float32)
		return ok && v >= -1234.5 && v <= 56789.123
	})
}
