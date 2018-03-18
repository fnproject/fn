package tests

import "reflect"

var updateConfigCases = []struct {
	name         string
	intialConfig map[string]string
	change       map[string]string
	expected     map[string]string
}{
	{"preserve existing config keys with nop", map[string]string{"key": "value1"}, map[string]string{}, map[string]string{"key": "value1"}},

	{"preserve existing config keys with change", map[string]string{"key": "value1"}, map[string]string{"key": "value1"}, map[string]string{"key": "value1"}},

	{"overwrite existing config keys", map[string]string{"key": "value1"}, map[string]string{"key": "value2"}, map[string]string{"key": "value2"}},

	{"delete config key", map[string]string{"key": "value1"}, map[string]string{"key": ""}, map[string]string{}},
}

func ConfigEquivalent(a map[string]string, b map[string]string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)

}
