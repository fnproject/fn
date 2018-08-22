package gopter_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
)

type counterShrink struct {
	n int
}

func (c *counterShrink) Next() (interface{}, bool) {
	if c.n > 0 {
		v := c.n
		c.n--
		return v, true
	}
	return 0, false
}

func TestShinkAll(t *testing.T) {
	counter := &counterShrink{n: 10}
	shrink := gopter.Shrink(counter.Next)

	all := shrink.All()
	if !reflect.DeepEqual(all, []interface{}{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}) {
		t.Errorf("Invalid all: %#v", all)
	}
}

func TestShrinkFilter(t *testing.T) {
	counter := &counterShrink{n: 20}
	shrink := gopter.Shrink(counter.Next)

	all := shrink.Filter(func(v interface{}) bool {
		return v.(int)%2 == 0
	}).All()
	if !reflect.DeepEqual(all, []interface{}{20, 18, 16, 14, 12, 10, 8, 6, 4, 2}) {
		t.Errorf("Invalid all: %#v", all)
	}

	counter = &counterShrink{n: 5}
	shrink = gopter.Shrink(counter.Next)

	all = shrink.Filter(nil).All()
	if !reflect.DeepEqual(all, []interface{}{5, 4, 3, 2, 1}) {
		t.Errorf("Invalid all: %#v", all)
	}
}

func TestShrinkConcat(t *testing.T) {
	counterShrink1 := &counterShrink{n: 5}
	counterShrink2 := &counterShrink{n: 4}
	shrink1 := gopter.Shrink(counterShrink1.Next)
	shrink2 := gopter.Shrink(counterShrink2.Next)

	all := gopter.ConcatShrinks(shrink1, shrink2).All()
	if !reflect.DeepEqual(all, []interface{}{5, 4, 3, 2, 1, 4, 3, 2, 1}) {
		t.Errorf("Invalid all: %#v", all)
	}
}

func TestShrinkInterleave(t *testing.T) {
	counterShrink1 := &counterShrink{n: 5}
	counterShrink2 := &counterShrink{n: 7}

	shrink1 := gopter.Shrink(counterShrink1.Next)
	shrink2 := gopter.Shrink(counterShrink2.Next)

	all := shrink1.Interleave(shrink2).All()
	if !reflect.DeepEqual(all, []interface{}{5, 7, 4, 6, 3, 5, 2, 4, 1, 3, 2, 1}) {
		t.Errorf("Invalid all: %#v", all)
	}
}

func TestCombineShrinker(t *testing.T) {
	var shrinker1Arg, shrinker2Arg interface{}
	shrinker1 := func(v interface{}) gopter.Shrink {
		shrinker1Arg = v
		shrink := &counterShrink{n: 5}
		return shrink.Next
	}
	shrinker2 := func(v interface{}) gopter.Shrink {
		shrinker2Arg = v
		shrink := &counterShrink{n: 3}
		return shrink.Next
	}
	shrinker := gopter.CombineShrinker(shrinker1, shrinker2)
	all := shrinker([]interface{}{123, 456}).All()
	if shrinker1Arg != 123 {
		t.Errorf("Invalid shrinker1Arg: %#v", shrinker1Arg)
	}
	if shrinker2Arg != 456 {
		t.Errorf("Invalid shrinker1Arg: %#v", shrinker1Arg)
	}
	if !reflect.DeepEqual(all, []interface{}{
		[]interface{}{5, 456},
		[]interface{}{4, 456},
		[]interface{}{3, 456},
		[]interface{}{2, 456},
		[]interface{}{1, 456},
		[]interface{}{123, 3},
		[]interface{}{123, 2},
		[]interface{}{123, 1},
	}) {
		t.Errorf("Invalid all: %#v", all)
	}
}

func TestShrinkMap(t *testing.T) {
	counter := &counterShrink{n: 10}
	shrink := gopter.Shrink(counter.Next).Map(func(v int) int {
		return 10 - v
	})

	all := shrink.All()
	if !reflect.DeepEqual(all, []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Errorf("Invalid all: %#v", all)
	}
}

func TestShrinkMapNoFunc(t *testing.T) {
	defer expectPanic(t, "Param of Map has to be a func, but is string")
	counter := &counterShrink{n: 10}
	gopter.Shrink(counter.Next).Map("not a function")
}

func TestShrinkMapTooManyParams(t *testing.T) {
	defer expectPanic(t, "Param of Map has to be a func with one param, but is 2")
	counter := &counterShrink{n: 10}
	gopter.Shrink(counter.Next).Map(func(a, b string) string {
		return ""
	})
}

func TestShrinkMapToManyReturns(t *testing.T) {
	defer expectPanic(t, "Param of Map has to be a func with one return value, but is 2")
	counter := &counterShrink{n: 10}
	gopter.Shrink(counter.Next).Map(func(a string) (string, bool) {
		return "", false
	})
}

func TestNoShrinker(t *testing.T) {
	shrink := gopter.NoShrinker(123)
	if shrink == nil {
		t.Error("Shrink has to be != nil")
	}
	value, ok := shrink()
	if ok || value != nil {
		t.Errorf("Invalid shrink: %#v", value)
	}
}
