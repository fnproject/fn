package gen

import (
	"reflect"

	"github.com/leanovate/gopter"
)

// ArrayOfN generates an array of generated elements with definied length
func ArrayOfN(desiredlen int, elementGen gopter.Gen, typeOverrides ...reflect.Type) gopter.Gen {
	var typeOverride reflect.Type
	if len(typeOverrides) > 1 {
		panic("too many type overrides specified, at most 1 may be provided.")
	} else if len(typeOverrides) == 1 {
		typeOverride = typeOverrides[0]
	}
	return func(genParams *gopter.GenParameters) *gopter.GenResult {
		result, elementSieve, elementShrinker := genArray(elementGen, genParams, desiredlen, typeOverride)

		genResult := gopter.NewGenResult(result.Interface(), ArrayShrinkerOne(elementShrinker))
		if elementSieve != nil {
			genResult.Sieve = func(v interface{}) bool {
				rv := reflect.ValueOf(v)
				return rv.Len() == desiredlen && forAllSieve(elementSieve)(v)
			}
		} else {
			genResult.Sieve = func(v interface{}) bool {
				return reflect.ValueOf(v).Len() == desiredlen
			}
		}
		return genResult
	}
}

func genArray(elementGen gopter.Gen, genParams *gopter.GenParameters, desiredlen int, typeOverride reflect.Type) (reflect.Value, func(interface{}) bool, gopter.Shrinker) {
	element := elementGen(genParams)
	elementSieve := element.Sieve
	elementShrinker := element.Shrinker

	sliceType := typeOverride
	if sliceType == nil {
		sliceType = element.ResultType
	}

	arrayType := reflect.ArrayOf(desiredlen, sliceType)
	result := reflect.New(arrayType).Elem()

	for i := 0; i < desiredlen; i++ {
		value, ok := element.Retrieve()

		if ok {
			if value == nil {
				result.Index(i).Set(reflect.Zero(sliceType))
			} else {
				result.Index(i).Set(reflect.ValueOf(value))
			}
		}
		element = elementGen(genParams)
	}

	return result, elementSieve, elementShrinker
}
