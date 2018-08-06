package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestMapShrinkerOne(t *testing.T) {
	mapShrink := gen.MapShrinkerOne(gen.StringShrinker, gen.Int64Shrinker)(map[string]int64{
		"two": 2,
	}).All()
	if !reflect.DeepEqual(mapShrink, []interface{}{
		map[string]int64{"wo": 2},
		map[string]int64{"wo": 0},
		map[string]int64{"to": 0},
		map[string]int64{"to": 1},
		map[string]int64{"tw": 1},
		map[string]int64{"tw": -1},
	}) {
		t.Errorf("Invalid mapShrink: %#v", mapShrink)
	}
}

func TestMapShrinker(t *testing.T) {
	mapShrink := gen.MapShrinker(gen.StringShrinker, gen.Int64Shrinker)(map[string]int64{
		"two": 2,
	}).All()
	if !reflect.DeepEqual(mapShrink, []interface{}{
		map[string]int64{"wo": 2},
		map[string]int64{"wo": 0},
		map[string]int64{"to": 0},
		map[string]int64{"to": 1},
		map[string]int64{"tw": 1},
		map[string]int64{"tw": -1},
	}) {
		t.Errorf("Invalid mapShrink: %#v", mapShrink)
	}

	mapShrink2 := gen.MapShrinker(gen.StringShrinker, gen.Int64Shrinker)(map[string]int64{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  3,
	}).All()

	if len(mapShrink2) < 10 {
		t.Errorf("mapShrink2 too short: %#v", mapShrink2)
	}
	for _, shrink := range mapShrink2 {
		_, ok := shrink.(map[string]int64)
		if !ok {
			t.Errorf("mapShrink2 invalid type: %#v", mapShrink2)
		}
	}
}
