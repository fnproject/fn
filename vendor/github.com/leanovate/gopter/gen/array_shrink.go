package gen

import (
	"fmt"
	"reflect"

	"github.com/leanovate/gopter"
)

type arrayShrinkOne struct {
	original      reflect.Value
	index         int
	elementShrink gopter.Shrink
}

func (s *arrayShrinkOne) Next() (interface{}, bool) {
	value, ok := s.elementShrink()
	if !ok {
		return nil, false
	}
	result := reflect.New(s.original.Type()).Elem()
	reflect.Copy(result, s.original)
	if value == nil {
		result.Index(s.index).Set(reflect.Zero(s.original.Type().Elem()))
	} else {
		result.Index(s.index).Set(reflect.ValueOf(value))
	}

	return result.Interface(), true
}

// ArrayShrinkerOne creates an array shrinker from a shrinker for the elements of the slice.
// The length of the array will remains unchanged, instead each element is shrunk after the
// other.
func ArrayShrinkerOne(elementShrinker gopter.Shrinker) gopter.Shrinker {
	return func(v interface{}) gopter.Shrink {
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Array {
			panic(fmt.Sprintf("%#v is not an array", v))
		}

		shrinks := make([]gopter.Shrink, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			arrayShrinkOne := &arrayShrinkOne{
				original:      rv,
				index:         i,
				elementShrink: elementShrinker(rv.Index(i).Interface()),
			}
			shrinks = append(shrinks, arrayShrinkOne.Next)
		}
		return gopter.ConcatShrinks(shrinks...)
	}
}
