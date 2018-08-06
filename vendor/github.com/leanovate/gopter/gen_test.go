package gopter_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
)

func constGen(value interface{}) gopter.Gen {
	return func(*gopter.GenParameters) *gopter.GenResult {
		return gopter.NewGenResult(value, gopter.NoShrinker)
	}
}

func TestGenSample(t *testing.T) {
	gen := constGen("sample")

	value, ok := gen.Sample()
	if !ok || value != "sample" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
}

func BenchmarkMap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gen := constGen("sample")
		var mappedWith string
		mapper := func(v string) string {
			mappedWith = v
			return "other"
		}
		value, ok := gen.Map(mapper).Sample()
		if !ok || value != "other" {
			b.Errorf("Invalid gen sample: %#v", value)
		}
		if mappedWith != "sample" {
			b.Errorf("Invalid mapped with: %#v", mappedWith)
		}

		gen = gen.SuchThat(func(interface{}) bool {
			return false
		})
		value, ok = gen.Map(mapper).Sample()
		if ok {
			b.Errorf("Invalid gen sample: %#v", value)
		}
	}
}

func TestGenMap(t *testing.T) {
	gen := constGen("sample")
	var mappedWith string
	mapper := func(v string) string {
		mappedWith = v
		return "other"
	}
	value, ok := gen.Map(mapper).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith != "sample" {
		t.Errorf("Invalid mapped with: %#v", mappedWith)
	}

	gen = gen.SuchThat(func(interface{}) bool {
		return false
	})
	value, ok = gen.Map(mapper).Sample()
	if ok {
		t.Errorf("Invalid gen sample: %#v", value)
	}
}

func TestGenMapWithParams(t *testing.T) {
	gen := constGen("sample")
	var mappedWith string
	var mappedWithParams *gopter.GenParameters
	mapper := func(v string, params *gopter.GenParameters) string {
		mappedWith = v
		mappedWithParams = params
		return "other"
	}
	value, ok := gen.Map(mapper).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith != "sample" {
		t.Errorf("Invalid mapped with: %#v", mappedWith)
	}
	if mappedWithParams == nil || mappedWithParams.MaxSize != 100 {
		t.Error("Mapper not called with currect parameters")
	}

	gen = gen.SuchThat(func(interface{}) bool {
		return false
	})
	value, ok = gen.Map(mapper).Sample()
	if ok {
		t.Errorf("Invalid gen sample: %#v", value)
	}
}

func TestGenMapNoFunc(t *testing.T) {
	defer expectPanic(t, "Param of Map has to be a func, but is string")
	constGen("sample").Map("not a function")
}

func TestGenMapTooManyParams(t *testing.T) {
	defer expectPanic(t, "Param of Map has to be a func with one or two params, but is 3")
	constGen("sample").Map(func(a, b, C string) string {
		return ""
	})
}

func TestGenMapInvalidSecondParam(t *testing.T) {
	defer expectPanic(t, "Second parameter of mapper function has to be a *GenParameters")
	constGen("sample").Map(func(a, b string) string {
		return ""
	})
}

func TestGenMapToInvalidParamtype(t *testing.T) {
	defer expectPanic(t, "Param of Map has to be a func with one param assignable to string, but is int")
	constGen("sample").Map(func(a int) string {
		return ""
	})
}

func TestGenMapToManyReturns(t *testing.T) {
	defer expectPanic(t, "Param of Map has to be a func with one return value, but is 2")
	constGen("sample").Map(func(a string) (string, bool) {
		return "", false
	})
}

func TestGenMapResultIn(t *testing.T) {
	gen := constGen("sample")
	var mappedWith *gopter.GenResult
	mapper := func(result *gopter.GenResult) string {
		mappedWith = result
		return "other"
	}

	value, ok := gen.Map(mapper).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith == nil {
		t.Error("Mapper not called")
	}
	if mapperValue, ok := mappedWith.Retrieve(); !ok || mapperValue != "sample" {
		t.Errorf("Mapper was called with invalid value: %#v", mapperValue)
	}
}

func TestGenMapResultInWithParams(t *testing.T) {
	gen := constGen("sample")
	var mappedWith *gopter.GenResult
	var mappedWithParams *gopter.GenParameters
	mapper := func(result *gopter.GenResult, params *gopter.GenParameters) string {
		mappedWith = result
		mappedWithParams = params
		return "other"
	}

	value, ok := gen.Map(mapper).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith == nil {
		t.Error("Mapper not called")
	}
	if mappedWithParams == nil || mappedWithParams.MaxSize != 100 {
		t.Error("Mapper not called with currect parameters")
	}
	if mapperValue, ok := mappedWith.Retrieve(); !ok || mapperValue != "sample" {
		t.Errorf("Mapper was called with invalid value: %#v", mapperValue)
	}
}

func TestGenMapResultOut(t *testing.T) {
	gen := constGen("sample")
	var mappedWith string
	mapper := func(v string) *gopter.GenResult {
		mappedWith = v
		return gopter.NewGenResult("other", gopter.NoShrinker)
	}
	value, ok := gen.Map(mapper).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith != "sample" {
		t.Errorf("Invalid mapped with: %#v", mappedWith)
	}

	gen = gen.SuchThat(func(interface{}) bool {
		return false
	})
	value, ok = gen.Map(mapper).Sample()
	if ok {
		t.Errorf("Invalid gen sample: %#v", value)
	}
}

func TestGenMapResultInOut(t *testing.T) {
	gen := constGen("sample")
	var mappedWith *gopter.GenResult
	mapper := func(result *gopter.GenResult) *gopter.GenResult {
		mappedWith = result
		return gopter.NewGenResult("other", gopter.NoShrinker)
	}

	value, ok := gen.Map(mapper).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith == nil {
		t.Error("Mapper not called")
	}
	if mapperValue, ok := mappedWith.Retrieve(); !ok || mapperValue != "sample" {
		t.Errorf("Mapper was called with invalid value: %#v", mapperValue)
	}
}

func TestGenFlatMap(t *testing.T) {
	gen := constGen("sample")
	var mappedWith interface{}
	mapper := func(v interface{}) gopter.Gen {
		mappedWith = v
		return constGen("other")
	}
	value, ok := gen.FlatMap(mapper, reflect.TypeOf("")).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith.(string) != "sample" {
		t.Errorf("Invalid mapped with: %#v", mappedWith)
	}

	gen = gen.SuchThat(func(interface{}) bool {
		return false
	})
	value, ok = gen.FlatMap(mapper, reflect.TypeOf("")).Sample()
	if ok {
		t.Errorf("Invalid gen sample: %#v", value)
	}
}

func TestGenMapResult(t *testing.T) {
	gen := constGen("sample")
	var mappedWith *gopter.GenResult
	mapper := func(result *gopter.GenResult) *gopter.GenResult {
		mappedWith = result
		return gopter.NewGenResult("other", gopter.NoShrinker)
	}

	value, ok := gen.MapResult(mapper).Sample()
	if !ok || value != "other" {
		t.Errorf("Invalid gen sample: %#v", value)
	}
	if mappedWith == nil {
		t.Error("Mapper not called")
	}
	if mapperValue, ok := mappedWith.Retrieve(); !ok || mapperValue != "sample" {
		t.Errorf("Mapper was called with invalid value: %#v", mapperValue)
	}
}

func TestCombineGens(t *testing.T) {
	gens := make([]gopter.Gen, 0, 20)
	for i := 0; i < 20; i++ {
		gens = append(gens, constGen(i))
	}
	gen := gopter.CombineGens(gens...)
	raw, ok := gen.Sample()
	if !ok {
		t.Errorf("Invalid combined gen: %#v", raw)
	}
	values, ok := raw.([]interface{})
	if !ok || !reflect.DeepEqual(values, []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19}) {
		t.Errorf("Invalid combined gen: %#v", raw)
	}

	gens[0] = gens[0].SuchThat(func(interface{}) bool {
		return false
	})
	gen = gopter.CombineGens(gens...)
	raw, ok = gen.Sample()
	if ok {
		t.Errorf("Invalid combined gen: %#v", raw)
	}
}

func TestSuchThat(t *testing.T) {
	var sieveArg string
	sieve := func(v string) bool {
		sieveArg = v
		return true
	}
	gen := constGen("sample").SuchThat(sieve)
	value, ok := gen.Sample()
	if !ok || value != "sample" {
		t.Errorf("Invalid result: %#v", value)
	}
	if sieveArg != "sample" {
		t.Errorf("Invalid sieveArg: %#v", sieveArg)
	}

	sieveArg = ""
	var sieve2Arg string
	sieve2 := func(v string) bool {
		sieve2Arg = v
		return false
	}
	gen = gen.SuchThat(sieve2)
	_, ok = gen.Sample()
	if ok {
		t.Error("Did not expect a result")
	}
	if sieveArg != "sample" {
		t.Errorf("Invalid sieveArg: %#v", sieveArg)
	}
	if sieve2Arg != "sample" {
		t.Errorf("Invalid sieve2Arg: %#v", sieve2Arg)
	}
}

func TestGenSuchThatNoFunc(t *testing.T) {
	defer expectPanic(t, "Param of SuchThat has to be a func, but is string")
	constGen("sample").SuchThat("not a function")
}

func TestGenSuchTooManyParams(t *testing.T) {
	defer expectPanic(t, "Param of SuchThat has to be a func with one param, but is 2")
	constGen("sample").SuchThat(func(a, b string) bool {
		return false
	})
}

func TestGenSuchThatToInvalidParamtype(t *testing.T) {
	defer expectPanic(t, "Param of SuchThat has to be a func with one param assignable to string, but is int")
	constGen("sample").SuchThat(func(a int) bool {
		return false
	})
}

func TestGenSuchToManyReturns(t *testing.T) {
	defer expectPanic(t, "Param of SuchThat has to be a func with one return value, but is 2")
	constGen("sample").SuchThat(func(a string) (string, bool) {
		return "", false
	})
}

func TestGenSuchToInvalidReturns(t *testing.T) {
	defer expectPanic(t, "Param of SuchThat has to be a func with one return value of bool, but is string")
	constGen("sample").SuchThat(func(a string) string {
		return ""
	})
}

func TestWithShrinker(t *testing.T) {
	var shrinkerArg interface{}
	shrinker := func(v interface{}) gopter.Shrink {
		shrinkerArg = v
		return gopter.NoShrink
	}
	gen := constGen("sample").WithShrinker(shrinker)
	result := gen(gopter.DefaultGenParameters())
	value, ok := result.Retrieve()
	if !ok {
		t.Errorf("Invalid combined value: %#v", value)
	}
	result.Shrinker(value)
	if shrinkerArg != "sample" {
		t.Errorf("Invalid shrinkerArg: %#v", shrinkerArg)
	}
}

func expectPanic(t *testing.T, expected string) {
	r := recover()
	if r == nil {
		t.Errorf("The code did not panic")
	} else if r.(string) != expected {
		t.Errorf("Panic does not match: '%#v' != '%#v'", r, expected)
	}
}
