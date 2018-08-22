package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

type testStruct struct {
	Value1 string
	Value2 int64
	Value3 []int8
	Value4 string
}

func TestStruct(t *testing.T) {
	structGen := gen.Struct(reflect.TypeOf(&testStruct{}), map[string]gopter.Gen{
		"Value1":   gen.Identifier(),
		"Value2":   gen.Int64(),
		"Value3":   gen.SliceOf(gen.Int8()),
		"NotThere": gen.AnyString(),
	})
	for i := 0; i < 100; i++ {
		value, ok := structGen.Sample()

		if !ok {
			t.Errorf("Invalid value: %#v", value)
		}
		v, ok := value.(testStruct)
		if !ok || v.Value1 == "" || v.Value3 == nil || v.Value4 != "" {
			t.Errorf("Invalid value: %#v", value)
		}
	}
}

func TestStructPropageEmpty(t *testing.T) {
	fail := gen.Struct(reflect.TypeOf(&testStruct{}), map[string]gopter.Gen{
		"Value1": gen.Identifier().SuchThat(func(str string) bool {
			return false
		}),
	})

	if _, ok := fail.Sample(); ok {
		t.Errorf("Failing field generator in Struct generated a value")
	}
}

func TestStructNoStruct(t *testing.T) {
	fail := gen.Struct(reflect.TypeOf(""), map[string]gopter.Gen{})

	if _, ok := fail.Sample(); ok {
		t.Errorf("Invalid Struct generated a value")
	}
}

func TestStructPtr(t *testing.T) {
	structGen := gen.StructPtr(reflect.TypeOf(&testStruct{}), map[string]gopter.Gen{
		"Value1":   gen.Identifier(),
		"Value2":   gen.Int64(),
		"Value3":   gen.SliceOf(gen.Int8()),
		"NotThere": gen.AnyString(),
	})
	for i := 0; i < 100; i++ {
		value, ok := structGen.Sample()

		if !ok || value == nil {
			t.Errorf("Invalid value: %#v", value)
		}
		v, ok := value.(*testStruct)
		if !ok || v.Value1 == "" || v.Value3 == nil || v.Value4 != "" {
			t.Errorf("Invalid value: %#v", value)
		}
	}
}

func TestStructPtrPropageEmpty(t *testing.T) {
	fail := gen.StructPtr(reflect.TypeOf(&testStruct{}), map[string]gopter.Gen{
		"Value1": gen.Identifier().SuchThat(func(str string) bool {
			return false
		}),
	})

	if _, ok := fail.Sample(); ok {
		t.Errorf("Failing field generator in StructPtr generated a value")
	}
}

func TestStructPtrNoStruct(t *testing.T) {
	fail := gen.StructPtr(reflect.TypeOf(""), map[string]gopter.Gen{})

	if _, ok := fail.Sample(); ok {
		t.Errorf("Invalid StructPtr generated a value")
	}
}
