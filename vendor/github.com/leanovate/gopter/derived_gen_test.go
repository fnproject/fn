package gopter_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

type downStruct struct {
	a int
	b string
	c bool
}

func TestDeriveGenSingleDown(t *testing.T) {
	gen := gopter.DeriveGen(
		func(a int, b string, c bool) *downStruct {
			return &downStruct{a: a, b: b, c: c}
		},
		func(d *downStruct) (int, string, bool) {
			return d.a, d.b, d.c
		},
		gen.Int(),
		gen.AnyString(),
		gen.Bool(),
	)

	sample, ok := gen.Sample()
	if !ok {
		t.Error("Sample not ok")
	}
	_, ok = sample.(*downStruct)
	if !ok {
		t.Errorf("%#v is not a downStruct", sample)
	}

	shrinker := gen(gopter.DefaultGenParameters()).Shrinker
	shrink := shrinker(&downStruct{a: 10, b: "abcd", c: false})

	shrinkedStructs := make([]*downStruct, 0)
	value, next := shrink()
	for next {
		shrinkedStruct, ok := value.(*downStruct)
		if !ok {
			t.Errorf("Invalid shrinked value: %#v", value)
		}
		shrinkedStructs = append(shrinkedStructs, shrinkedStruct)
		value, next = shrink()
	}

	expected := []*downStruct{
		&downStruct{a: 0, b: "abcd", c: false},
		&downStruct{a: 5, b: "abcd", c: false},
		&downStruct{a: -5, b: "abcd", c: false},
		&downStruct{a: 8, b: "abcd", c: false},
		&downStruct{a: -8, b: "abcd", c: false},
		&downStruct{a: 9, b: "abcd", c: false},
		&downStruct{a: -9, b: "abcd", c: false},
		&downStruct{a: 10, b: "cd", c: false},
		&downStruct{a: 10, b: "ab", c: false},
		&downStruct{a: 10, b: "bcd", c: false},
		&downStruct{a: 10, b: "acd", c: false},
		&downStruct{a: 10, b: "abd", c: false},
		&downStruct{a: 10, b: "abc", c: false},
	}
	if !reflect.DeepEqual(shrinkedStructs, expected) {
		t.Errorf("%v does not equal %v", shrinkedStructs, expected)
	}
}

func TestDeriveGenSingleDownWithSieves(t *testing.T) {
	gen := gopter.DeriveGen(
		func(a int, b string, c bool) *downStruct {
			return &downStruct{a: a, b: b, c: c}
		},
		func(d *downStruct) (int, string, bool) {
			return d.a, d.b, d.c
		},
		gen.Int().SuchThat(func(i int) bool {
			return i%2 == 0
		}),
		gen.AnyString(),
		gen.Bool(),
	)

	parameters := gopter.DefaultGenParameters()
	parameters.Rng.Seed(1234)

	hasNoValue := false
	for i := 0; i < 100; i++ {
		result := gen(parameters)
		_, ok := result.Retrieve()
		if !ok {
			hasNoValue = true
			break
		}
	}
	if !hasNoValue {
		t.Error("Sieve is not applied")
	}

	sieve := gen(parameters).Sieve

	if !sieve(&downStruct{a: 2, b: "something", c: false}) {
		t.Error("Sieve did not pass even")
	}

	if sieve(&downStruct{a: 3, b: "something", c: false}) {
		t.Error("Sieve did pass odd")
	}
}

func TestDeriveGenMultiDown(t *testing.T) {
	gen := gopter.DeriveGen(
		func(a int, b string, c bool, d int32) (*downStruct, int64) {
			return &downStruct{a: a, b: b, c: c}, int64(a) + int64(d)
		},
		func(d *downStruct, diff int64) (int, string, bool, int32) {
			return d.a, d.b, d.c, int32(diff - int64(d.a))
		},
		gen.Int(),
		gen.AnyString(),
		gen.Bool(),
		gen.Int32(),
	)

	sample, ok := gen.Sample()
	if !ok {
		t.Error("Sample not ok")
	}
	values, ok := sample.([]interface{})
	if !ok || len(values) != 2 {
		t.Errorf("%#v is not a slice of interface", sample)
	}
	_, ok = values[0].(*downStruct)
	if !ok {
		t.Errorf("%#v is not a downStruct", values[0])
	}
	_, ok = values[1].(int64)
	if !ok {
		t.Errorf("%#v is not a int64", values[1])
	}

	shrinker := gen(gopter.DefaultGenParameters()).Shrinker
	shrink := shrinker([]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(20)})

	value, next := shrink()
	shrinkedValues := make([][]interface{}, 0)
	for next {
		shrinked, ok := value.([]interface{})
		if !ok || len(values) != 2 {
			t.Errorf("%#v is not a slice of interface", sample)
		}
		shrinkedValues = append(shrinkedValues, shrinked)
		value, next = shrink()
	}

	expected := [][]interface{}{
		[]interface{}{&downStruct{a: 0, b: "abcd", c: false}, int64(10)},
		[]interface{}{&downStruct{a: 5, b: "abcd", c: false}, int64(15)},
		[]interface{}{&downStruct{a: -5, b: "abcd", c: false}, int64(5)},
		[]interface{}{&downStruct{a: 8, b: "abcd", c: false}, int64(18)},
		[]interface{}{&downStruct{a: -8, b: "abcd", c: false}, int64(2)},
		[]interface{}{&downStruct{a: 9, b: "abcd", c: false}, int64(19)},
		[]interface{}{&downStruct{a: -9, b: "abcd", c: false}, int64(1)},
		[]interface{}{&downStruct{a: 10, b: "cd", c: false}, int64(20)},
		[]interface{}{&downStruct{a: 10, b: "ab", c: false}, int64(20)},
		[]interface{}{&downStruct{a: 10, b: "bcd", c: false}, int64(20)},
		[]interface{}{&downStruct{a: 10, b: "acd", c: false}, int64(20)},
		[]interface{}{&downStruct{a: 10, b: "abd", c: false}, int64(20)},
		[]interface{}{&downStruct{a: 10, b: "abc", c: false}, int64(20)},
		[]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(10)},
		[]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(15)},
		[]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(5)},
		[]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(18)},
		[]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(2)},
		[]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(19)},
		[]interface{}{&downStruct{a: 10, b: "abcd", c: false}, int64(1)},
	}

	if !reflect.DeepEqual(shrinkedValues, expected) {
		t.Errorf("%v does not equal %v", shrinkedValues, expected)
	}
}
