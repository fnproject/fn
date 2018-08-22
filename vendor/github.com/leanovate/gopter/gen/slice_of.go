package gen

import (
	"reflect"

	"github.com/leanovate/gopter"
)

// SliceOf generates an arbitrary slice of generated elements
// genParams.MaxSize sets an (exclusive) upper limit on the size of the slice
// genParams.MinSize sets an (inclusive) lower limit on the size of the slice
func SliceOf(elementGen gopter.Gen) gopter.Gen {
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		len := 0
		if genParams.MaxSize > 0 || genParams.MinSize > 0 {
			if genParams.MinSize > genParams.MaxSize {
				panic("GenParameters.MinSize must be <= GenParameters.MaxSize")
			}

			if genParams.MaxSize == genParams.MinSize {
				len = genParams.MaxSize
			} else {
				len = genParams.Rng.Intn(genParams.MaxSize-genParams.MinSize) + genParams.MinSize
			}
		}
		result, elementSieve, elementShrinker := genSlice(elementGen, genParams, len)

		genResult := gopter.NewGenResult(result.Interface(), SliceShrinker(elementShrinker))
		if elementSieve != nil {
			genResult.Sieve = forAllSieve(elementSieve)
		}
		return genResult
	}
}

// SliceOfN generates a slice of generated elements with definied length
func SliceOfN(len int, elementGen gopter.Gen) gopter.Gen {
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		result, elementSieve, elementShrinker := genSlice(elementGen, genParams, len)

		genResult := gopter.NewGenResult(result.Interface(), SliceShrinkerOne(elementShrinker))
		if elementSieve != nil {
			genResult.Sieve = func(v interface{}) bool {
				rv := reflect.ValueOf(v)
				return rv.Len() == len && forAllSieve(elementSieve)(v)
			}
		} else {
			genResult.Sieve = func(v interface{}) bool {
				return reflect.ValueOf(v).Len() == len
			}
		}
		return genResult
	}
}

func genSlice(elementGen gopter.Gen, genParams *gopter.GenParameters, len int) (reflect.Value, func(interface{}) bool, gopter.Shrinker) {
	element := elementGen(genParams)
	elementSieve := element.Sieve
	elementShrinker := element.Shrinker

	result := reflect.MakeSlice(reflect.SliceOf(element.ResultType), 0, len)

	for i := 0; i < len; i++ {
		value, ok := element.Retrieve()

		if ok {
			if value == nil {
				result = reflect.Append(result, reflect.Zero(element.ResultType))
			} else {
				result = reflect.Append(result, reflect.ValueOf(value))
			}
		}
		element = elementGen(genParams)
	}

	return result, elementSieve, elementShrinker
}

func forAllSieve(elementSieve func(interface{}) bool) func(interface{}) bool {
	return func(v interface{}) bool {
		rv := reflect.ValueOf(v)
		for i := rv.Len() - 1; i >= 0; i-- {
			if !elementSieve(rv.Index(i).Interface()) {
				return false
			}
		}
		return true
	}
}
