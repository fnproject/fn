package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestPtrShrinker(t *testing.T) {
	v := 10
	shinks := []int{0, 5, -5, 8, -8, 9, -9}
	intPtrShrink := gen.PtrShrinker(gen.IntShrinker)(&v).All()
	if !reflect.DeepEqual(intPtrShrink, []interface{}{nil, &shinks[0], &shinks[1], &shinks[2], &shinks[3], &shinks[4], &shinks[5], &shinks[6]}) {
		t.Errorf("Invalid intPtrShrink: %#v", intPtrShrink)
	}
}
