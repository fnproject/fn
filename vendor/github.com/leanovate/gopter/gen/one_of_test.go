package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

func TestOneConstOf(t *testing.T) {
	consts := gen.OneConstOf("one", "two", "three", "four")
	commonOneOfTest(t, consts)

	fail := gen.OneConstOf()
	if _, ok := fail.Sample(); ok {
		t.Errorf("Empty OneConstOf generated a value")
	}
}

func TestOneGenOf(t *testing.T) {
	consts := gen.OneGenOf(gen.Const("one"), gen.Const("two"), gen.Const("three"), gen.Const("four"))
	commonOneOfTest(t, consts)

	fail := gen.OneGenOf()
	if _, ok := fail.Sample(); ok {
		t.Errorf("Empty OneGenOf generated a value")
	}
}

func commonOneOfTest(t *testing.T, gen gopter.Gen) {
	generated := make(map[string]bool, 0)
	for i := 0; i < 100; i++ {
		value, ok := gen.Sample()

		if !ok || value == nil {
			t.Errorf("Invalid consts: %#v", value)
		}
		v, ok := value.(string)
		if !ok {
			t.Errorf("Invalid consts: %#v", value)
		}
		generated[v] = true
	}
	if !reflect.DeepEqual(generated, map[string]bool{
		"one":   true,
		"two":   true,
		"three": true,
		"four":  true,
	}) {
		t.Errorf("Not all consts where generated: %#v", generated)
	}
}
