package gen_test

import (
	"math"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

func TestInt64Range(t *testing.T) {
	fail := gen.Int64Range(200, 100)

	if value, ok := fail.Sample(); value != nil || ok {
		t.Fail()
	}

	commonGeneratorTest(t, "int 64 range", gen.Int64Range(-123456, 234567), func(value interface{}) bool {
		v, ok := value.(int64)
		return ok && v >= -123456 || v <= 234567
	})

	commonGeneratorTest(t, "int 64 positive", gen.Int64Range(1, math.MaxInt64), func(value interface{}) bool {
		v, ok := value.(int64)
		return ok && v > 0
	})

	commonGeneratorTest(t, "int 64 negative", gen.Int64Range(math.MinInt64, -1), func(value interface{}) bool {
		v, ok := value.(int64)
		return ok && v < 0
	})

	commonGeneratorTest(t, "full int 64 range", gen.Int64Range(math.MinInt64, math.MaxInt64), func(value interface{}) bool {
		_, ok := value.(int64)
		return ok
	})
}

func TestUInt64Range(t *testing.T) {
	fail := gen.UInt64Range(200, 100)

	if value, ok := fail.Sample(); value != nil || ok {
		t.Fail()
	}

	commonGeneratorTest(t, "uint 64 range", gen.UInt64Range(0, 234567), func(value interface{}) bool {
		v, ok := value.(uint64)
		return ok && v <= 234567
	})
}

func TestInt64(t *testing.T) {
	commonGeneratorTest(t, "int 64", gen.Int64(), func(value interface{}) bool {
		_, ok := value.(int64)
		return ok
	})

	commonGeneratorTest(t, "uint 64", gen.UInt64(), func(value interface{}) bool {
		_, ok := value.(uint64)
		return ok
	})
}

func TestInt32(t *testing.T) {
	commonGeneratorTest(t, "int 32", gen.Int32(), func(value interface{}) bool {
		_, ok := value.(int32)
		return ok
	})

	commonGeneratorTest(t, "uint 32", gen.UInt32(), func(value interface{}) bool {
		_, ok := value.(uint32)
		return ok
	})
}

func TestInt16(t *testing.T) {
	commonGeneratorTest(t, "int 16", gen.Int16(), func(value interface{}) bool {
		_, ok := value.(int16)
		return ok
	})

	commonGeneratorTest(t, "uint 16", gen.UInt16(), func(value interface{}) bool {
		_, ok := value.(uint16)
		return ok
	})
}

func TestInt8(t *testing.T) {
	commonGeneratorTest(t, "int 8", gen.Int8(), func(value interface{}) bool {
		_, ok := value.(int8)
		return ok
	})

	commonGeneratorTest(t, "uint 8", gen.UInt8(), func(value interface{}) bool {
		_, ok := value.(uint8)
		return ok
	})
}

func TestInt(t *testing.T) {
	commonGeneratorTest(t, "int", gen.Int(), func(value interface{}) bool {
		_, ok := value.(int)
		return ok
	})
	commonGeneratorTest(t, "intrange", gen.IntRange(-1234, 5678), func(value interface{}) bool {
		v, ok := value.(int)
		return ok && v >= -1234 && v <= 5678
	})

	commonGeneratorTest(t, "uint", gen.UInt(), func(value interface{}) bool {
		_, ok := value.(uint)
		return ok
	})
	commonGeneratorTest(t, "uintrange", gen.UIntRange(1234, 5678), func(value interface{}) bool {
		v, ok := value.(uint)
		return ok && v >= 1234 && v <= 5678
	})
}

func TestGenSize(t *testing.T) {
	params := gopter.DefaultGenParameters()
	genSize := gen.Size()
	for i := 0; i < 100; i++ {
		result := genSize(params.WithSize(i))
		value, ok := result.Retrieve()
		if !ok || value.(int) != i {
			t.Errorf("Invalid gen size: %v", value)
		}
	}
}
