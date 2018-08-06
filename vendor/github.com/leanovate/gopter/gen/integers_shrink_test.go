package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestInt64Shrink(t *testing.T) {
	zeroShrinks := gen.Int64Shrinker(int64(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.Int64Shrinker(int64(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{int64(0), int64(5), int64(-5), int64(8), int64(-8), int64(9), int64(-9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}

	negTenShrinks := gen.Int64Shrinker(int64(-10)).All()
	if !reflect.DeepEqual(negTenShrinks, []interface{}{int64(0), int64(-5), int64(5), int64(-8), int64(8), int64(-9), int64(9)}) {
		t.Errorf("Invalid negTenShrinks: %#v", negTenShrinks)
	}

	leetShrink := gen.Int64Shrinker(int64(1337)).All()
	if !reflect.DeepEqual(leetShrink, []interface{}{
		int64(0), int64(669), int64(-669), int64(1003), int64(-1003), int64(1170), int64(-1170),
		int64(1254), int64(-1254), int64(1296), int64(-1296), int64(1317), int64(-1317),
		int64(1327), int64(-1327), int64(1332), int64(-1332), int64(1335), int64(-1335),
		int64(1336), int64(-1336)}) {
		t.Errorf("Invalid leetShrink: %#v", leetShrink)
	}
}

func TestUInt64Shrink(t *testing.T) {
	zeroShrinks := gen.UInt64Shrinker(uint64(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.UInt64Shrinker(uint64(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{uint64(0), uint64(5), uint64(8), uint64(9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}

	leetShrink := gen.UInt64Shrinker(uint64(1337)).All()
	if !reflect.DeepEqual(leetShrink, []interface{}{
		uint64(0), uint64(669), uint64(1003), uint64(1170),
		uint64(1254), uint64(1296), uint64(1317),
		uint64(1327), uint64(1332), uint64(1335),
		uint64(1336)}) {
		t.Errorf("Invalid leetShrink: %#v", leetShrink)
	}
}

func TestInt32Shrink(t *testing.T) {
	zeroShrinks := gen.Int32Shrinker(int32(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.Int32Shrinker(int32(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{int32(0), int32(5), int32(-5), int32(8), int32(-8), int32(9), int32(-9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}
}

func TestUInt32Shrink(t *testing.T) {
	zeroShrinks := gen.UInt32Shrinker(uint32(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.UInt32Shrinker(uint32(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{uint32(0), uint32(5), uint32(8), uint32(9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}
}

func TestInt16Shrink(t *testing.T) {
	zeroShrinks := gen.Int16Shrinker(int16(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.Int16Shrinker(int16(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{int16(0), int16(5), int16(-5), int16(8), int16(-8), int16(9), int16(-9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}
}

func TestUInt16Shrink(t *testing.T) {
	zeroShrinks := gen.UInt16Shrinker(uint16(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.UInt16Shrinker(uint16(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{uint16(0), uint16(5), uint16(8), uint16(9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}
}

func TestInt8Shrink(t *testing.T) {
	zeroShrinks := gen.Int8Shrinker(int8(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.Int8Shrinker(int8(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{int8(0), int8(5), int8(-5), int8(8), int8(-8), int8(9), int8(-9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}
}

func TestUInt8Shrink(t *testing.T) {
	zeroShrinks := gen.UInt8Shrinker(uint8(0)).All()
	if !reflect.DeepEqual(zeroShrinks, []interface{}{}) {
		t.Errorf("Invalid zeroShrinks: %#v", zeroShrinks)
	}

	tenShrinks := gen.UInt8Shrinker(uint8(10)).All()
	if !reflect.DeepEqual(tenShrinks, []interface{}{uint8(0), uint8(5), uint8(8), uint8(9)}) {
		t.Errorf("Invalid tenShrinks: %#v", tenShrinks)
	}
}
