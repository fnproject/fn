package tests

import (
	"fmt"
	"reflect"
	"strings"
)

const (
	maxMetadataKeys = 100
	maxValueSize    = 512
	maxKeysize      = 128
)

var emptyMd = map[string]interface{}{}

func makeMdMap(size int) map[string]interface{} {
	md := make(map[string]interface{}, size)
	for i := 0; i < size; i++ {
		md[fmt.Sprintf("k-%d", i)] = "val"
	}
	return md
}

var createMetadataValidCases = []struct {
	name     string
	metadata map[string]interface{}
}{
	{"valid_string", map[string]interface{}{"key": "value"}},
	{"valid_array", map[string]interface{}{"key": []interface{}{"value1", "value2"}}},
	{"valid_object", map[string]interface{}{"key": map[string]interface{}{"foo": "bar"}}},
	{"max_value_size", map[string]interface{}{"key": strings.Repeat("a", maxValueSize-2)}},
	{"max_key_size", map[string]interface{}{strings.Repeat("a", maxKeysize): "val"}},
	{"max_map_size", makeMdMap(maxMetadataKeys)},
}

var createMetadataErrorCases = []struct {
	name     string
	metadata map[string]interface{}
}{
	{"value_too_long", map[string]interface{}{"key": strings.Repeat("a", maxValueSize-1)}},
	{"key_too_long", map[string]interface{}{strings.Repeat("a", maxKeysize+1): "value"}},
	{"whitespace_in_key", map[string]interface{}{" bad key ": "value"}},
	{"too_many_keys", makeMdMap(maxMetadataKeys + 1)},
}

var updateMetadataValidCases = []struct {
	name            string
	initialMetadata map[string]interface{}
	change          map[string]interface{}
	expected        map[string]interface{}
}{
	{"overwrite_existing_metadata_keys", map[string]interface{}{"key": "value1"}, map[string]interface{}{"key": "value2"}, map[string]interface{}{"key": "value2"}},
	{"delete_metadata_key", map[string]interface{}{"key": "value1"}, map[string]interface{}{"key": ""}, map[string]interface{}{}},
	{"set_to_max_size_with_deletes", map[string]interface{}{"key": "value1"}, func() map[string]interface{} {
		md := makeMdMap(100)
		md["key"] = ""
		return md
	}(), makeMdMap(100)},
	{"noop_with_max_keys", makeMdMap(maxMetadataKeys), emptyMd, makeMdMap(maxMetadataKeys)},
}

var updateMetadataErrorCases = []struct {
	name            string
	initialMetadata map[string]interface{}
	change          map[string]interface{}
}{
	{"too_many_key_after_update", makeMdMap(100), map[string]interface{}{"key": "value1"}},
	{"value_too_long", map[string]interface{}{}, map[string]interface{}{"key": strings.Repeat("a", maxValueSize-1)}},
	{"key_too_long", map[string]interface{}{}, map[string]interface{}{strings.Repeat("a", maxKeysize+1): "value"}},
	{"whitespace_in_key", map[string]interface{}{}, map[string]interface{}{" bad key ": "value"}},
	{"too_many_keys_in_update", map[string]interface{}{}, makeMdMap(maxMetadataKeys + 1)},
}

func MetadataEquivalent(md1, md2 map[string]interface{}) bool {

	if len(md1) == 0 && len(md2) == 0 {
		return true
	}
	return reflect.DeepEqual(md1, md2)
}
