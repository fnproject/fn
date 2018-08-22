package arbitrary

import (
	"reflect"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

func mapBoolish(to reflect.Type, v interface{}) interface{} {
	value := reflect.ValueOf(v)
	result := reflect.New(to).Elem()
	result.SetBool(value.Bool())
	return result.Interface()
}

func mapIntish(to reflect.Type, v interface{}) interface{} {
	value := reflect.ValueOf(v)
	result := reflect.New(to).Elem()
	result.SetInt(value.Int())
	return result.Interface()
}

func mapUintish(to reflect.Type, v interface{}) interface{} {
	value := reflect.ValueOf(v)
	result := reflect.New(to).Elem()
	result.SetUint(value.Uint())
	return result.Interface()
}

func mapFloatish(to reflect.Type, v interface{}) interface{} {
	value := reflect.ValueOf(v)
	result := reflect.New(to).Elem()
	result.SetFloat(value.Float())
	return result.Interface()
}

func mapComplexish(to reflect.Type, v interface{}) interface{} {
	value := reflect.ValueOf(v)
	result := reflect.New(to).Elem()
	result.SetComplex(value.Complex())
	return result.Interface()
}

func mapStringish(to reflect.Type, v interface{}) interface{} {
	value := reflect.ValueOf(v)
	result := reflect.New(to).Elem()
	result.SetString(value.String())
	return result.Interface()
}

func (a *Arbitraries) genForKind(rt reflect.Type) gopter.Gen {
	switch rt.Kind() {
	case reflect.Bool:
		return gen.Bool().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapBoolish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapBoolish(reflect.TypeOf(bool(false)), v))
				},
				Shrinker: gopter.NoShrinker,
			}
		})
	case reflect.Int:
		return gen.Int().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapIntish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapIntish(reflect.TypeOf(int(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapIntish(reflect.TypeOf(int(0)), v)).Map(func(s interface{}) interface{} {
						return mapIntish(rt, s)
					})
				},
			}
		})
	case reflect.Uint:
		return gen.UInt().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapUintish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapUintish(reflect.TypeOf(uint(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapUintish(reflect.TypeOf(uint(0)), v)).Map(func(s interface{}) interface{} {
						return mapUintish(rt, s)
					})
				},
			}
		})
	case reflect.Int8:
		return gen.Int8().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapIntish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapIntish(reflect.TypeOf(int8(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapIntish(reflect.TypeOf(int8(0)), v)).Map(func(s interface{}) interface{} {
						return mapIntish(rt, s)
					})
				},
			}
		})
	case reflect.Uint8:
		return gen.UInt8().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapUintish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapUintish(reflect.TypeOf(uint8(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapUintish(reflect.TypeOf(uint8(0)), v)).Map(func(s interface{}) interface{} {
						return mapUintish(rt, s)
					})
				},
			}
		})
	case reflect.Int16:
		return gen.Int16().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapIntish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapIntish(reflect.TypeOf(int16(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapIntish(reflect.TypeOf(int16(0)), v)).Map(func(s interface{}) interface{} {
						return mapIntish(rt, s)
					})
				},
			}
		})
	case reflect.Uint16:
		return gen.UInt16().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapUintish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapUintish(reflect.TypeOf(uint16(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapUintish(reflect.TypeOf(uint16(0)), v)).Map(func(s interface{}) interface{} {
						return mapUintish(rt, s)
					})
				},
			}
		})
	case reflect.Int32:
		return gen.Int32().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapIntish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapIntish(reflect.TypeOf(int32(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapIntish(reflect.TypeOf(int32(0)), v)).Map(func(s interface{}) interface{} {
						return mapIntish(rt, s)
					})
				},
			}
		})
	case reflect.Uint32:
		return gen.UInt32().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapUintish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapUintish(reflect.TypeOf(uint32(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapUintish(reflect.TypeOf(uint32(0)), v)).Map(func(s interface{}) interface{} {
						return mapUintish(rt, s)
					})
				},
			}
		})
	case reflect.Int64:
		return gen.Int64().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapIntish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapIntish(reflect.TypeOf(int32(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapIntish(reflect.TypeOf(int64(0)), v)).Map(func(s interface{}) interface{} {
						return mapIntish(rt, s)
					})
				},
			}
		})
	case reflect.Uint64:
		return gen.UInt64().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapUintish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapUintish(reflect.TypeOf(uint64(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapUintish(reflect.TypeOf(uint64(0)), v)).Map(func(s interface{}) interface{} {
						return mapUintish(rt, s)
					})
				},
			}
		})
	case reflect.Float32:
		return gen.Float32().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapFloatish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapFloatish(reflect.TypeOf(float32(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapFloatish(reflect.TypeOf(float32(0)), v)).Map(func(s interface{}) interface{} {
						return mapFloatish(rt, s)
					})
				},
			}
		})
	case reflect.Float64:
		return gen.Float64().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapFloatish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapFloatish(reflect.TypeOf(float64(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapFloatish(reflect.TypeOf(float64(0)), v)).Map(func(s interface{}) interface{} {
						return mapFloatish(rt, s)
					})
				},
			}
		})
	case reflect.Complex64:
		return gen.Complex64().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapComplexish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapComplexish(reflect.TypeOf(complex64(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapComplexish(reflect.TypeOf(complex64(0)), v)).Map(func(s interface{}) interface{} {
						return mapComplexish(rt, s)
					})
				},
			}
		})
	case reflect.Complex128:
		return gen.Complex128().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapComplexish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapComplexish(reflect.TypeOf(complex128(0)), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapComplexish(reflect.TypeOf(complex128(0)), v)).Map(func(s interface{}) interface{} {
						return mapComplexish(rt, s)
					})
				},
			}
		})
	case reflect.String:
		return gen.AnyString().MapResult(func(result *gopter.GenResult) *gopter.GenResult {
			return &gopter.GenResult{
				Labels:     result.Labels,
				ResultType: rt,
				Result:     mapStringish(rt, result.Result),
				Sieve: func(v interface{}) bool {
					return result.Sieve == nil || result.Sieve(mapStringish(reflect.TypeOf(string("")), v))
				},
				Shrinker: func(v interface{}) gopter.Shrink {
					return result.Shrinker(mapStringish(reflect.TypeOf(string("")), v)).Map(func(s interface{}) interface{} {
						return mapStringish(rt, s)
					})
				},
			}
		})
	case reflect.Slice:
		if elementGen := a.GenForType(rt.Elem()); elementGen != nil {
			return gen.SliceOf(elementGen)
		}
	case reflect.Ptr:
		if rt.Elem().Kind() == reflect.Struct {
			gens := make(map[string]gopter.Gen)
			for i := 0; i < rt.Elem().NumField(); i++ {
				field := rt.Elem().Field(i)
				if gen := a.GenForType(field.Type); gen != nil {
					gens[field.Name] = gen
				}
			}
			return gen.StructPtr(rt, gens)
		}
		return gen.PtrOf(a.GenForType(rt.Elem()))
	case reflect.Struct:
		gens := make(map[string]gopter.Gen)
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			if gen := a.GenForType(field.Type); gen != nil {
				gens[field.Name] = gen
			}
		}
		return gen.Struct(rt, gens)
	}
	return nil
}
