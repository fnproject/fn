package gopter_test

import (
	"testing"

	"github.com/leanovate/gopter"
)

func TestBiMapperParamNotMatch(t *testing.T) {
	defer expectPanic(t, "upstream has wrong parameter type 0: string != int")
	gopter.NewBiMapper(func(int) int { return 0 }, func(string) int { return 0 })
}

func TestBiMapperReturnNotMatch(t *testing.T) {
	defer expectPanic(t, "upstream has wrong return type 0: string != int")
	gopter.NewBiMapper(func(int) int { return 0 }, func(int) string { return "" })
}

func TestBiMapperInvalidDownstream(t *testing.T) {
	defer expectPanic(t, "downstream has to be a function")
	gopter.NewBiMapper(1, 2)
}

func TestBiMapperInvalidUpstream(t *testing.T) {
	defer expectPanic(t, "upstream has to be a function")
	gopter.NewBiMapper(func(int) int { return 0 }, 2)
}
