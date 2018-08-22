package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestSliceShrink(t *testing.T) {
	oneShrink := gen.SliceShrinker(gen.Int64Shrinker)([]int64{0}).All()
	if !reflect.DeepEqual(oneShrink, []interface{}{}) {
		t.Errorf("Invalid oneShrink: %#v", oneShrink)
	}

	twoShrink := gen.SliceShrinker(gen.Int64Shrinker)([]int64{0, 1}).All()
	if !reflect.DeepEqual(twoShrink, []interface{}{
		[]int64{1},
		[]int64{0},
		[]int64{0, 0},
	}) {
		t.Errorf("Invalid twoShrink: %#v", twoShrink)
	}

	threeShrink := gen.SliceShrinker(gen.Int64Shrinker)([]int64{0, 1, 2}).All()
	if !reflect.DeepEqual(threeShrink, []interface{}{
		[]int64{1, 2},
		[]int64{0, 2},
		[]int64{0, 1},
		[]int64{0, 0, 2},
		[]int64{0, 1, 0},
		[]int64{0, 1, 1},
		[]int64{0, 1, -1},
	}) {
		t.Errorf("Invalid threeShrink: %#v", threeShrink)
	}

	fourShrink := gen.SliceShrinker(gen.Int64Shrinker)([]int64{0, 1, 2, 3}).All()
	if !reflect.DeepEqual(fourShrink, []interface{}{
		[]int64{2, 3},
		[]int64{0, 1},
		[]int64{1, 2, 3},
		[]int64{0, 2, 3},
		[]int64{0, 1, 3},
		[]int64{0, 1, 2},
		[]int64{0, 0, 2, 3},
		[]int64{0, 1, 0, 3},
		[]int64{0, 1, 1, 3},
		[]int64{0, 1, -1, 3},
		[]int64{0, 1, 2, 0},
		[]int64{0, 1, 2, 2},
		[]int64{0, 1, 2, -2},
	}) {
		t.Errorf("Invalid fourShrink: %#v", fourShrink)
	}
}

func TestSliceShrinkOne(t *testing.T) {
	oneShrink := gen.SliceShrinkerOne(gen.Int64Shrinker)([]int64{0}).All()
	if !reflect.DeepEqual(oneShrink, []interface{}{}) {
		t.Errorf("Invalid oneShrink: %#v", oneShrink)
	}

	threeShrink := gen.SliceShrinkerOne(gen.Int64Shrinker)([]int64{0, 1, 2}).All()
	if !reflect.DeepEqual(threeShrink, []interface{}{
		[]int64{0, 0, 2},
		[]int64{0, 1, 0},
		[]int64{0, 1, 1},
		[]int64{0, 1, -1},
	}) {
		t.Errorf("Invalid threeShrink: %#v", threeShrink)
	}

	fourShrink := gen.SliceShrinkerOne(gen.Int64Shrinker)([]int64{0, 1, 2, 3}).All()
	if !reflect.DeepEqual(fourShrink, []interface{}{
		[]int64{0, 0, 2, 3},
		[]int64{0, 1, 0, 3},
		[]int64{0, 1, 1, 3},
		[]int64{0, 1, -1, 3},
		[]int64{0, 1, 2, 0},
		[]int64{0, 1, 2, 2},
		[]int64{0, 1, 2, -2},
	}) {
		t.Errorf("Invalid fourShrink: %#v", fourShrink)
	}
}
