package gen_test

import (
	"math"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestComplex128Box(t *testing.T) {
	minReal := -12345.67
	maxReal := 2345.78
	minImag := -5432.8
	maxImag := 8764.6
	complexs := gen.Complex128Box(complex(minReal, minImag), complex(maxReal, maxImag))
	commonGeneratorTest(t, "complex 128 box", complexs, func(value interface{}) bool {
		v, ok := value.(complex128)
		return ok && real(v) >= minReal && real(v) < maxReal && imag(v) >= minImag && imag(v) < maxImag
	})
}

func TestComplex128(t *testing.T) {
	commonGeneratorTest(t, "complex 128", gen.Complex128(), func(value interface{}) bool {
		v, ok := value.(complex128)
		return ok && !math.IsNaN(real(v)) && !math.IsNaN(imag(v)) && !math.IsInf(real(v), 0) && !math.IsInf(imag(v), 0)
	})
}

func TestComplex64Box(t *testing.T) {
	minReal := float32(-12345.67)
	maxReal := float32(2345.78)
	minImag := float32(-5432.8)
	maxImag := float32(8764.6)
	complexs := gen.Complex64Box(complex(minReal, minImag), complex(maxReal, maxImag))
	commonGeneratorTest(t, "complex 64 box", complexs, func(value interface{}) bool {
		v, ok := value.(complex64)
		return ok && real(v) >= minReal && real(v) < maxReal && imag(v) >= minImag && imag(v) < maxImag
	})
}

func TestComplex64(t *testing.T) {
	commonGeneratorTest(t, "complex 64", gen.Complex64(), func(value interface{}) bool {
		_, ok := value.(complex64)
		return ok
	})
}
