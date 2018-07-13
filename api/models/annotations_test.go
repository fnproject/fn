package models

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type testObj struct {
	Md Annotations `json:"annotations,omitempty"`
}

type myJson struct {
	Foo string `json:"foo,omitempty"`
	Bar string `json:"bar,omitempty"`
}

func (m Annotations) withRawKey(key string, val string) Annotations {
	newMd := make(Annotations)
	for k, v := range m {
		newMd[k] = v
	}

	v := annotationValue([]byte(val))
	newMd[key] = &v
	return newMd
}

func mustParseMd(t *testing.T, md string) Annotations {
	mdObj := make(Annotations)

	err := json.Unmarshal([]byte(md), &mdObj)
	if err != nil {
		t.Fatalf("Failed to parse must-parse value %s %v", md, err)
	}
	return mdObj
}

func TestAnnotationsEqual(t *testing.T) {
	annWithVal, _ := EmptyAnnotations().With("foo", "Bar")

	tcs := []struct {
		a      Annotations
		b      Annotations
		equals bool
	}{
		{EmptyAnnotations(), EmptyAnnotations(), true},
		{annWithVal, EmptyAnnotations(), false},
		{annWithVal, annWithVal, true},
		{EmptyAnnotations().withRawKey("v1", `"a"`), EmptyAnnotations().withRawKey("v1", `"b"`), false},
		{EmptyAnnotations().withRawKey("v1", `"a"`), EmptyAnnotations().withRawKey("v2", `"a"`), false},

		{annWithVal.Without("foo"), EmptyAnnotations(), true},
		{mustParseMd(t,
			"{ \r\n\t"+`"md.1":{ `+"\r\n\t"+`

			"subkey1": "value\n with\n newlines",

			"subkey2": true
		}
		}`), mustParseMd(t, `{"md.1":{"subkey1":"value\n with\n newlines", "subkey2":true}}`), true},
	}

	for _, tc := range tcs {
		if tc.a.Equals(tc.b) != tc.equals {
			t.Errorf("Annotations equality mismatch - expecting (%v == %v) = %v", tc.b, tc.a, tc.equals)
		}
		if tc.b.Equals(tc.a) != tc.equals {
			t.Errorf("Annotations reflexive equality mismatch - expecting (%v == %v) = %v", tc.b, tc.a, tc.equals)
		}
	}

}

var annCases = []struct {
	val       *testObj
	valString string
}{
	{val: &testObj{Md: EmptyAnnotations()}, valString: "{}"},
	{val: &testObj{Md: EmptyAnnotations().withRawKey("stringval", `"bar"`)}, valString: `{"annotations":{"stringval":"bar"}}`},
	{val: &testObj{Md: EmptyAnnotations().withRawKey("intval", `1001`)}, valString: `{"annotations":{"intval":1001}}`},
	{val: &testObj{Md: EmptyAnnotations().withRawKey("floatval", "3.141")}, valString: `{"annotations":{"floatval":3.141}}`},
	{val: &testObj{Md: EmptyAnnotations().withRawKey("objval", `{"foo":"fooval","bar":"barval"}`)}, valString: `{"annotations":{"objval":{"foo":"fooval","bar":"barval"}}}`},
	{val: &testObj{Md: EmptyAnnotations().withRawKey("objval", `{"foo":"fooval","bar":{"barbar":"barbarval"}}`)}, valString: `{"annotations":{"objval":{"foo":"fooval","bar":{"barbar":"barbarval"}}}}`},
	{val: &testObj{Md: EmptyAnnotations().withRawKey("objval", `{"foo":"JSON newline \\n string"}`)}, valString: `{"annotations":{"objval":{"foo":"JSON newline \\n string"}}}`},
}

func TestAnnotationsJSONMarshalling(t *testing.T) {

	for _, tc := range annCases {
		v, err := json.Marshal(tc.val)
		if err != nil {
			t.Fatalf("Failed to marshal json into %s: %v", tc.valString, err)
		}
		if string(v) != tc.valString {
			t.Errorf("Invalid annotations value, expected %s, got %s", tc.valString, string(v))
		}
	}

}

func TestAnnotationsJSONUnMarshalling(t *testing.T) {

	for _, tc := range annCases {
		tv := testObj{}
		err := json.Unmarshal([]byte(tc.valString), &tv)
		if err != nil {
			t.Fatalf("Failed to unmarshal json into %s: %v", tc.valString, err)
		}
		if !reflect.DeepEqual(&tv, tc.val) {
			t.Errorf("Invalid annotations value, expected %v, got %v", tc.val, tv)
		}
	}

}

func TestAnnotationsWithHonorsKeyLimits(t *testing.T) {
	var validKeys = []string{
		"ok",
		strings.Repeat("a", maxAnnotationKeyBytes),
		"fnproject/internal/foo",
		"foo.bar.com.baz",
		"foo$bar!_+-()[]:@/<>$",
	}
	for _, k := range validKeys {
		m, err := EmptyAnnotations().With(k, "value")

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
		{"", ErrInvalidAnnotationKey},
		{" ", ErrInvalidAnnotationKey},
		{"\u00e9", ErrInvalidAnnotationKey},
		{"foo bar", ErrInvalidAnnotationKey},
		{strings.Repeat("a", maxAnnotationKeyBytes+1), ErrInvalidAnnotationKeyLength},
	}
	for _, kc := range invalidKeys {
		_, err := EmptyAnnotations().With(kc.key, "value")
		if err == nil {
			t.Errorf("Should have rejected invalid key %s", kc.key)
		}

		m := EmptyAnnotations().withRawKey(kc.key, "\"data\"")
		err = m.Validate()
		if err != kc.err {
			t.Errorf("Should have returned validation error  %v,  for key %s got %v", kc.err, kc.key, err)
		}

	}

}

func TestAnnotationsHonorsValueLimits(t *testing.T) {
	validValues := []interface{}{
		"ok",
		&myJson{Foo: "foo"},
		strings.Repeat("a", maxAnnotationValueBytes-2),
		[]string{strings.Repeat("a", maxAnnotationValueBytes-4)},

		1,
		[]string{"a", "b", "c"},
		true,
	}

	for _, v := range validValues {

		_, err := EmptyAnnotations().With("key", v)
		if err != nil {
			t.Errorf("Should have accepted valid value %s,%v", v, err)
		}

		rawJson, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		md := EmptyAnnotations().withRawKey("key", string(rawJson))

		err = md.Validate()
		if err != nil {
			t.Errorf("Should have validated valid value successfully %s, got error %v", string(rawJson), err)
		}
	}

	invalidValues := []struct {
		val interface{}
		err APIError
	}{
		{"", ErrInvalidAnnotationValue},
		{nil, ErrInvalidAnnotationValue},
		{strings.Repeat("a", maxAnnotationValueBytes-1), ErrInvalidAnnotationValueLength},
		{[]string{strings.Repeat("a", maxAnnotationValueBytes-3)}, ErrInvalidAnnotationValueLength},
	}

	for _, v := range invalidValues {
		_, err := EmptyAnnotations().With("key", v.val)
		if err == nil {
			t.Errorf("Should have rejected invalid value \"%v\"", v)
		}

		rawJson, err := json.Marshal(v.val)
		if err != nil {
			panic(err)
		}
		md := EmptyAnnotations().withRawKey("key", string(rawJson))

		err = md.Validate()
		if err != v.err {
			t.Errorf("Expected validation error %v for '%s', got %v", v.err, string(rawJson), err)
		}
	}

}

func TestMergeAnnotations(t *testing.T) {

	mdWithNKeys := func(n int) Annotations {
		md := EmptyAnnotations()
		for i := 0; i < n; i++ {
			md = md.withRawKey(fmt.Sprintf("key-%d", i), "val")
		}
		return md
	}
	validCases := []struct {
		first  Annotations
		second Annotations
		result Annotations
	}{
		{first: EmptyAnnotations(), second: EmptyAnnotations(), result: EmptyAnnotations()},
		{first: EmptyAnnotations().withRawKey("key1", "\"val\""), second: EmptyAnnotations(), result: EmptyAnnotations().withRawKey("key1", "\"val\"")},
		{first: EmptyAnnotations(), second: EmptyAnnotations().withRawKey("key1", "\"val\""), result: EmptyAnnotations().withRawKey("key1", "\"val\"")},
		{first: EmptyAnnotations().withRawKey("key1", "\"val\""), second: EmptyAnnotations().withRawKey("key1", "\"val\""), result: EmptyAnnotations().withRawKey("key1", "\"val\"")},
		{first: EmptyAnnotations().withRawKey("key1", "\"val1\""), second: EmptyAnnotations().withRawKey("key2", "\"val2\""), result: EmptyAnnotations().withRawKey("key1", "\"val1\"").withRawKey("key2", "\"val2\"")},
		{first: EmptyAnnotations().withRawKey("key1", "\"val1\""), second: EmptyAnnotations().withRawKey("key1", "\"\""), result: EmptyAnnotations()},
		{first: EmptyAnnotations().withRawKey("key1", "\"val1\""), second: EmptyAnnotations().withRawKey("key2", "\"\""), result: EmptyAnnotations().withRawKey("key1", "\"val1\"")},
		{first: mdWithNKeys(maxAnnotationsKeys - 1), second: EmptyAnnotations().withRawKey("newkey", "\"val\""), result: mdWithNKeys(maxAnnotationsKeys-1).withRawKey("newkey", "\"val\"")},
	}

	for _, v := range validCases {
		newMd := v.first.MergeChange(v.second)

		if !reflect.DeepEqual(newMd, v.result) {
			t.Errorf("Change %v + %v :  expected %v got %v", v.first, v.second, v.result, newMd)
		}

	}

}

func TestGetAnnotations(t *testing.T) {
	annotations := EmptyAnnotations()
	annotations, err := annotations.With("string-annotation", "string-value")
	if err != nil {
		t.Fatal("Cannot add string annotation")
	}
	annotations, err = annotations.With("array-annotation", []string{"string-1", "string-2"})
	if err != nil {
		t.Fatal("Cannot add array annotation")
	}
	strAnnotation, ok := annotations.Get("string-annotation")
	if !ok {
		t.Error("Cannot get string annotation")
	}
	expected := "\"string-value\""
	if string(strAnnotation) != expected {
		t.Errorf("Got unexpected value for string annotation. Got: %s Expected %s", strAnnotation, expected)
	}
	arrAnnotation, ok := annotations.Get("array-annotation")
	if !ok {
		t.Error("Cannot get array annotation")
	}
	expected = "[\"string-1\",\"string-2\"]"
	if string(arrAnnotation) != expected {
		t.Errorf("Got unexpected value for array annotation. Got: %s Expected %s", strAnnotation, expected)
	}

	stringAnnotation, err := annotations.GetString("string-annotation")
	if err != nil {
		t.Fatalf("Error decoding string annotation: %v", err)
	}
	expected = "string-value"
	if stringAnnotation != expected {
		t.Errorf("Got unexpected decoded value for string annotation. Got: %s Expected %s", strAnnotation, expected)
	}

	_, err = annotations.GetString("array-annotation")
	if err == nil {
		t.Error("Expected error trying to retrieve a string value for array annotation")
	}
}
