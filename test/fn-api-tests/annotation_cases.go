package tests

import (
	"fmt"
	"reflect"
	"strings"
)

//  common test cases around annotations (shared by any objects that support it)

const (
	maxAnnotationKeys      = 100
	maxAnnotationValueSize = 512
	maxAnnotationKeySize   = 128
)

var emptyAnnMap = map[string]interface{}{}

func makeAnnMap(size int) map[string]interface{} {
	md := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		md[fmt.Sprintf("k-%d", i)] = "val"
	}
	return md
}

var createAnnotationsValidCases = []struct {
	name        string
	annotations map[string]interface{}
}{
	{"valid_string", map[string]interface{}{"key": "value"}},
	{"valid_array", map[string]interface{}{"key": []interface{}{"value1", "value2"}}},
	{"valid_object", map[string]interface{}{"key": map[string]interface{}{"foo": "bar"}}},
	{"max_value_size", map[string]interface{}{"key": strings.Repeat("a", maxAnnotationValueSize-2)}},
	{"max_key_size", map[string]interface{}{strings.Repeat("a", maxAnnotationKeySize): "val"}},
	{"max_map_size", makeAnnMap(maxAnnotationKeys)},
}

var createAnnotationsErrorCases = []struct {
	name        string
	annotations map[string]interface{}
}{
	{"value_too_long", map[string]interface{}{"key": strings.Repeat("a", maxAnnotationValueSize-1)}},
	{"key_too_long", map[string]interface{}{strings.Repeat("a", maxAnnotationKeySize+1): "value"}},
	{"whitespace_in_key", map[string]interface{}{" bad key ": "value"}},
	{"too_many_keys", makeAnnMap(maxAnnotationKeys + 1)},
}

var updateAnnotationsValidCases = []struct {
	name     string
	initial  map[string]interface{}
	change   map[string]interface{}
	expected map[string]interface{}
}{
	{"overwrite_existing_annotation_keys", map[string]interface{}{"key": "value1"}, map[string]interface{}{"key": "value2"}, map[string]interface{}{"key": "value2"}},
	{"delete_annotation_key", map[string]interface{}{"key": "value1"}, map[string]interface{}{"key": ""}, map[string]interface{}{}},
	{"set_to_max_size_with_deletes", map[string]interface{}{"key": "value1"}, func() map[string]interface{} {
		md := makeAnnMap(100)
		md["key"] = ""
		return md
	}(), makeAnnMap(100)},
	{"noop_with_max_keys", makeAnnMap(maxAnnotationKeys), emptyAnnMap, makeAnnMap(maxAnnotationKeys)},
}

var updateAnnotationsErrorCases = []struct {
	name    string
	initial map[string]interface{}
	change  map[string]interface{}
}{
	{"too_many_key_after_update", makeAnnMap(100), map[string]interface{}{"key": "value1"}},
	{"value_too_long", map[string]interface{}{}, map[string]interface{}{"key": strings.Repeat("a", maxAnnotationValueSize-1)}},
	{"key_too_long", map[string]interface{}{}, map[string]interface{}{strings.Repeat("a", maxAnnotationKeySize+1): "value"}},
	{"whitespace_in_key", map[string]interface{}{}, map[string]interface{}{" bad key ": "value"}},
	{"too_many_keys_in_update", map[string]interface{}{}, makeAnnMap(maxAnnotationKeys + 1)},
}

//AnnotationsEquivalent checks if two annotations maps are semantically equivalent, including nil == empty map
func AnnotationsEquivalent(md1, md2 map[string]interface{}) bool {

	if len(md1) == 0 && len(md2) == 0 {
		return true
	}
	return reflect.DeepEqual(md1, md2)
}
