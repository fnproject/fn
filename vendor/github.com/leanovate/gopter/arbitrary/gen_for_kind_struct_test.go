package arbitrary_test

import (
	"reflect"
	"testing"
	"unicode"

	"github.com/leanovate/gopter/arbitrary"
	"github.com/leanovate/gopter/gen"
)

type DemoStruct struct {
	Value1 int64
	Value2 string
	Value3 []uint
	Value4 int32
}

func TestArbitrariesStructs(t *testing.T) {
	arbitraries := arbitrary.DefaultArbitraries()

	arbitraries.RegisterGen(gen.Int64Range(10, 100))
	arbitraries.RegisterGen(gen.Int32Range(1, 10))
	arbitraries.RegisterGen(gen.Const([]uint{1, 2, 3}))
	arbitraries.RegisterGen(gen.AlphaString())

	gen := arbitraries.GenForType(reflect.TypeOf(&DemoStruct{}))
	for i := 0; i < 100; i++ {
		raw, ok := gen.Sample()
		if !ok {
			t.Errorf("Invalid value: %#v", raw)
		}
		value, ok := raw.(*DemoStruct)
		if !ok {
			t.Errorf("Invalid value: %#v", raw)
		}
		if value.Value1 < 10 || value.Value1 > 100 {
			t.Errorf("Invalid value.Value1 out of bounds: %#v", raw)
		}
		for _, ch := range value.Value2 {
			if !unicode.IsLetter(ch) {
				t.Errorf("Invalid value.Value2: %#v", raw)
			}
		}
		if !reflect.DeepEqual(value.Value3, []uint{1, 2, 3}) {
			t.Errorf("Invalid value.Value3: %#v", raw)
		}
		if value.Value4 < 1 || value.Value4 > 10 {
			t.Errorf("Invalid value.Value4 out of bounds: %#v", raw)
		}
	}
}
