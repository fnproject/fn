package models

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type testObj struct {
	Md Metadata `json:"metadata,omitempty"`
}

type myJson struct {
	Foo string `json:"foo,omitempty"`
	Bar string `json:"bar,omitempty"`
}

func (m Metadata) withRawKey(key string, val string) Metadata {
	newMd := make(Metadata)
	for k, v := range m {
		newMd[k] = v
	}

	newMd[key] = &metadataValue{val: []byte(val)}
	return newMd
}

func TestMetadataEqual(t *testing.T) {
	mdWithVal, _ := EmptyMetadata().With("foo", "Bar")

	tcs := []struct {
		a      Metadata
		b      Metadata
		equals bool
	}{

		{EmptyMetadata(), EmptyMetadata(), true},
		{mdWithVal, EmptyMetadata(), false},
		{mdWithVal, mdWithVal, true},
		{mdWithVal.Without("foo"), EmptyMetadata(), true},
	}

	for _, tc := range tcs {
		if tc.a.Equals(tc.b) != tc.equals {
			t.Errorf("Metadata equality mismatch - expecting (%v == %v) = %v", tc.b, tc.a, tc.equals)
		}
		if tc.b.Equals(tc.a) != tc.equals {
			t.Errorf("Metadata reflexive equality mismatch - expecting (%v == %v) = %v", tc.b, tc.a, tc.equals)
		}
	}

}

var mdCases = []struct {
	val       *testObj
	valString string
}{
	{val: &testObj{Md: EmptyMetadata()}, valString: "{}"},
	{val: &testObj{Md: EmptyMetadata().withRawKey("stringval", "\"bar\"")}, valString: "{\"metadata\":{\"stringval\":\"bar\"}}"},
	{val: &testObj{Md: EmptyMetadata().withRawKey("intval", "1001")}, valString: "{\"metadata\":{\"intval\":1001}}"},
	{val: &testObj{Md: EmptyMetadata().withRawKey("floatval", "3.141")}, valString: "{\"metadata\":{\"floatval\":3.141}}"},
	{val: &testObj{Md: EmptyMetadata().withRawKey("objval", "{\"foo\":\"fooval\",\"bar\":\"barval\"}")}, valString: "{\"metadata\":{\"objval\":{\"foo\":\"fooval\",\"bar\":\"barval\"}}}"},
	{val: &testObj{Md: EmptyMetadata().withRawKey("objval", "{\"foo\":\"fooval\",\"bar\":{\"barbar\":\"barbarval\"}}")}, valString: "{\"metadata\":{\"objval\":{\"foo\":\"fooval\",\"bar\":{\"barbar\":\"barbarval\"}}}}"},
}

func TestMetadataJSONMarshalling(t *testing.T) {

	for _, tc := range mdCases {
		v, err := json.Marshal(tc.val)
		if err != nil {
			t.Fatalf("Failed to marshal json into %s: %v", tc.valString, err)
		}
		if string(v) != tc.valString {
			t.Errorf("Invalid metadata value, expected %s, got %s", tc.valString, string(v))
		}
	}

}

func TestMetadataJSONUnMarshalling(t *testing.T) {

	for _, tc := range mdCases {
		tv := testObj{}
		err := json.Unmarshal([]byte(tc.valString), &tv)
		if err != nil {
			t.Fatalf("Failed to unmarshal json into %s: %v", tc.valString, err)
		}
		if !reflect.DeepEqual(&tv, tc.val) {
			t.Errorf("Invalid metadata value, expected %v, got %v", tc.val, tv)
		}
	}

}

func TestMetadataWithHonorsKeyLimits(t *testing.T) {
	var validKeys = []string{
		"ok",
		strings.Repeat("a", maxMetadataKeyBytes),
		"fnproject/internal/foo",
		"foo.bar.com.baz",
		"foo$bar!_+-()[]:@/<>$",
	}
	for _, k := range validKeys {
		m, err := EmptyMetadata().With(k, "value")

		if err != nil {
			t.Errorf("Should have accepted valid key %s,%v", k, err)
		}

		err = m.Validate()
		if err != nil {
			t.Errorf("Should have validate valid key %s,%v", k, err)
		}

	}

	var invalidKeys = []struct {
		key string
		err APIError
	}{
		{"", ErrInvalidMetadataKey},
		{" ", ErrInvalidMetadataKey},
		{"\u00e9", ErrInvalidMetadataKey},
		{"foo bar", ErrInvalidMetadataKey},
		{strings.Repeat("a", maxMetadataKeyBytes+1), ErrInvalidMetadataKeyLength},
	}
	for _, kc := range invalidKeys {
		_, err := EmptyMetadata().With(kc.key, "value")
		if err == nil {
			t.Errorf("Should have rejected invalid key %s", kc.key)
		}

		m := EmptyMetadata().withRawKey(kc.key, "\"data\"")
		err = m.Validate()
		if err != kc.err {
			t.Errorf("Should have returned validation error  %v,  for key %s got %v", kc.err, kc.key, err)
		}

	}

}

func TestMetadataHonorsValueLimits(t *testing.T) {
	validValues := []interface{}{
		"ok",
		&myJson{Foo: "foo"},
		strings.Repeat("a", maxMetadataValueBytes-2),
		[]string{strings.Repeat("a", maxMetadataValueBytes-4)},

		1,
		[]string{"a", "b", "c"},
		true,
	}

	for _, v := range validValues {

		_, err := EmptyMetadata().With("key", v)
		if err != nil {
			t.Errorf("Should have accepted valid value %s,%v", v, err)
		}

		rawJson, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		md := EmptyMetadata().withRawKey("key", string(rawJson))

		err = md.Validate()
		if err != nil {
			t.Errorf("Should have validated valid value  successfully %s, got error %v", string(rawJson), err)
		}
	}

	invalidValues := []struct {
		val interface{}
		err APIError
	}{
		{"", ErrInvalidMetadataValue},
		{nil, ErrInvalidMetadataValue},
		{strings.Repeat("a", maxMetadataValueBytes-1), ErrInvalidMetadataValueLength},
		{[]string{strings.Repeat("a", maxMetadataValueBytes-3)}, ErrInvalidMetadataValueLength},
	}

	for _, v := range invalidValues {
		_, err := EmptyMetadata().With("key", v.val)
		if err == nil {
			t.Errorf("Should have rejected invalid value \"%v\"", v)
		}

		rawJson, err := json.Marshal(v.val)
		if err != nil {
			panic(err)
		}
		md := EmptyMetadata().withRawKey("key", string(rawJson))

		err = md.Validate()
		if err != v.err {
			t.Errorf("Expected validation error %v for '%s', got %v", v.err, string(rawJson), err)
		}
	}

}

func TestMergeMetadata(t *testing.T) {

	mdWithNKeys := func(n int) Metadata {
		md := EmptyMetadata()
		for i := 0; i < n; i++ {
			md = md.withRawKey(fmt.Sprintf("key-%d", i), "val")
		}
		return md
	}
	validCases := []struct {
		first  Metadata
		second Metadata
		result Metadata
	}{
		{first: EmptyMetadata(), second: EmptyMetadata(), result: EmptyMetadata()},
		{first: EmptyMetadata().withRawKey("key1", "\"val\""), second: EmptyMetadata(), result: EmptyMetadata().withRawKey("key1", "\"val\"")},
		{first: EmptyMetadata(), second: EmptyMetadata().withRawKey("key1", "\"val\""), result: EmptyMetadata().withRawKey("key1", "\"val\"")},
		{first: EmptyMetadata().withRawKey("key1", "\"val\""), second: EmptyMetadata().withRawKey("key1", "\"val\""), result: EmptyMetadata().withRawKey("key1", "\"val\"")},
		{first: EmptyMetadata().withRawKey("key1", "\"val1\""), second: EmptyMetadata().withRawKey("key2", "\"val2\""), result: EmptyMetadata().withRawKey("key1", "\"val1\"").withRawKey("key2", "\"val2\"")},
		{first: EmptyMetadata().withRawKey("key1", "\"val1\""), second: EmptyMetadata().withRawKey("key1", "\"\""), result: EmptyMetadata()},
		{first: EmptyMetadata().withRawKey("key1", "\"val1\""), second: EmptyMetadata().withRawKey("key2", "\"\""), result: EmptyMetadata().withRawKey("key1", "\"val1\"")},
		{first: mdWithNKeys(maxMetadataKeys - 1), second: EmptyMetadata().withRawKey("newkey", "\"val\""), result: mdWithNKeys(maxMetadataKeys-1).withRawKey("newkey", "\"val\"")},
	}

	for _, v := range validCases {
		newMd := v.first.MergeChange(v.second)

		if !reflect.DeepEqual(newMd, v.result) {
			t.Errorf("Change %v + %v :  expected %v got %v", v.first, v.second, v.result, newMd)
		}

	}

}
