package models

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

var openEmptyJSON = `{"id":"","name":"","app_id":"","fn_id":"","created_at":"0001-01-01T00:00:00.000Z","updated_at":"0001-01-01T00:00:00.000Z","type":"","source":""`

var triggerJSONCases = []struct {
	val       *Trigger
	valString string
}{
	{val: &Trigger{}, valString: openEmptyJSON + "}"},
}

func TestTriggerJsonMarshalling(t *testing.T) {
	for _, tc := range triggerJSONCases {
		v, err := json.Marshal(tc.val)
		if err != nil {
			t.Fatalf("Failed to marshal json into %s: %v", tc.valString, err)
		}
		if string(v) != tc.valString {
			t.Errorf("Invalid trigger value, expected %s, got %s", tc.valString, string(v))
		}
	}
}

func TestTriggerListJsonMarshalling(t *testing.T) {
	emptyList := &TriggerList{Items: []*Trigger{}}
	expected := "{\"items\":[]}"

	v, err := json.Marshal(emptyList)
	if err != nil {
		t.Fatalf("Failed to marshal json into %s: %v", expected, err)
	}
	if string(v) != expected {
		t.Errorf("Invalid trigger value, expected %s, got %s", expected, string(v))
	}
}

var httpTrigger = &Trigger{Name: "name", AppID: "foo", FnID: "bar", Type: "http", Source: "/baz"}
var invalidTrigger = &Trigger{Name: "name", AppID: "foo", FnID: "bar", Type: "error", Source: "/baz"}

var triggerValidateCases = []struct {
	val   *Trigger
	valid bool
}{
	{val: &Trigger{}, valid: false},
	{val: invalidTrigger, valid: false},
	{val: httpTrigger, valid: true},
}

func TestTriggerValidate(t *testing.T) {
	for _, tc := range triggerValidateCases {
		v := tc.val.Validate()
		if v != nil && tc.valid {
			t.Errorf("Expected Trigger to be valid, but err (%s) returned. Trigger: %#v", v, tc.val)
		}
		if v == nil && !tc.valid {
			t.Errorf("Expected Trigger to be invalid, but no err returned. Trigger: %#v", tc.val)
		}
	}
}

func triggerReflectType() reflect.Type {
	trigger := Trigger{}
	return reflect.TypeOf(trigger)
}

func triggerFieldGenerators(t *testing.T) map[string]gopter.Gen {
	fieldGens := make(map[string]gopter.Gen)
	fieldGens["ID"] = gen.AlphaString()
	fieldGens["Name"] = gen.AlphaString()
	fieldGens["AppID"] = gen.AlphaString()
	fieldGens["FnID"] = gen.AlphaString()
	fieldGens["CreatedAt"] = datetimeGenerator()
	fieldGens["UpdatedAt"] = datetimeGenerator()
	fieldGens["Type"] = gen.AlphaString()
	fieldGens["Source"] = gen.AlphaString()
	fieldGens["Annotations"] = annotationGenerator()

	triggerFieldCount := triggerReflectType().NumField()

	if triggerFieldCount != len(fieldGens) {
		t.Fatalf("Trigger struct field count, %d, does not match trigger generator field count, %d", triggerFieldCount, len(fieldGens))
	}

	return fieldGens
}

func triggerGenerator(t *testing.T) gopter.Gen {
	return gen.Struct(triggerReflectType(), triggerFieldGenerators(t))
}

func TestTriggerEquality(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("A trigger should always equal itself", prop.ForAll(
		func(trigger Trigger) bool {
			return trigger.Equals(&trigger)
		},
		triggerGenerator(t),
	))

	properties.Property("A trigger should always equal a clone of itself", prop.ForAll(
		func(trigger Trigger) bool {
			clone := trigger.Clone()
			return trigger.Equals(clone)
		},
		triggerGenerator(t),
	))

	triggerFieldGens := triggerFieldGenerators(t)

	properties.Property("A trigger should never match a modified version of itself", prop.ForAll(
		func(trigger Trigger) bool {
			for fieldName, fieldGen := range triggerFieldGens {

				if fieldName == "CreatedAt" ||
					fieldName == "UpdatedAt" {
					continue
				}

				currentValue, newValue := novelValue(t, reflect.ValueOf(trigger), fieldName, fieldGen)

				clone := trigger.Clone()
				s := reflect.ValueOf(clone).Elem()
				field := s.FieldByName(fieldName)

				field.Set(newValue)

				if trigger.Equals(clone) {
					t.Errorf("Changed field, %s, from {%v} to {%v}, but still equal.", fieldName, currentValue, newValue)
					return false
				}
			}
			return true
		},
		triggerGenerator(t),
	))

	properties.TestingRun(t)
}

func TestValidateTriggerName(t *testing.T) {
	testCases := []struct {
		Name string
		Want error
	}{
		{"valid_name-101", nil},
		{"a_trigger_with_a_name_that_is_too_long", ErrTriggerTooLongName},
		{"", ErrTriggerMissingName},
		{"invalid.character", ErrTriggerInvalidName},
	}

	for _, testCase := range testCases {
		trigger := Trigger{Name: testCase.Name}
		got := trigger.ValidateName()

		if got != testCase.Want {
			t.Errorf(
				"Trigger.ValidateName() failed for %q - wanted: %q but got: %q",
				testCase.Name, testCase.Want, got)
		}
	}
}

func TestValidateTrigger(t *testing.T) {
	type test struct {
		Trigger Trigger
		Want    error
	}

	testCases := []test{
		{
			Trigger: generateValidTrigger(),
			Want:    nil,
		},
	}

	// Generate valid Triggers then break them to check validation is work
	testTrigger := generateValidTrigger()
	testTrigger.Name = ""
	testCases = append(testCases, test{testTrigger, ErrTriggerMissingName})

	testTrigger = generateValidTrigger()
	testTrigger.AppID = ""
	testCases = append(testCases, test{testTrigger, ErrTriggerMissingAppID})

	testTrigger = generateValidTrigger()
	testTrigger.FnID = ""
	testCases = append(testCases, test{testTrigger, ErrTriggerMissingFnID})

	testTrigger = generateValidTrigger()
	testTrigger.Type = ""
	testCases = append(testCases, test{testTrigger, ErrTriggerTypeUnknown})

	testTrigger = generateValidTrigger()
	testTrigger.Source = ""
	testCases = append(testCases, test{testTrigger, ErrTriggerMissingSource})

	testTrigger = generateValidTrigger()
	testTrigger.Source = "invalid-src"
	testCases =
		append(testCases, test{testTrigger, ErrTriggerMissingSourcePrefix})

	for _, testCase := range testCases {
		got := testCase.Trigger.Validate()

		if got != testCase.Want {
			t.Errorf(
				"Trigger.Validate() failed for '%+v' - wanted: %q but got: %q",
				testCase.Trigger, testCase.Want, got)
		}
	}
}

// Generate a Trigger structure which passes validation
func generateValidTrigger() Trigger {
	return Trigger{
		Name:   "valid_name",
		AppID:  "valid_app",
		FnID:   "valid_fn_id",
		Type:   "http",
		Source: "/valid-src",
	}
}
