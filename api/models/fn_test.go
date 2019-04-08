package models

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func fnReflectType() reflect.Type {
	fn := Fn{}
	return reflect.TypeOf(fn)
}

func resourceConfigGenerator(t *testing.T) gopter.Gen {
	fieldGens := make(map[string]gopter.Gen)

	fieldGens["Memory"] = gen.UInt64()
	fieldGens["Timeout"] = gen.Int32()
	fieldGens["IdleTimeout"] = gen.Int32()

	resourceConfig := ResourceConfig{}
	resourceConfigFieldCount := reflect.TypeOf(resourceConfig).NumField()

	if resourceConfigFieldCount != len(fieldGens) {
		t.Fatalf("Fn struct field count, %d, does not match fn generator field count, %d", resourceConfigFieldCount, len(fieldGens))
	}

	return gen.Struct(reflect.TypeOf(resourceConfig), fieldGens)
}

func fnFieldGenerators(t *testing.T) map[string]gopter.Gen {
	fieldGens := make(map[string]gopter.Gen)

	fieldGens["ID"] = gen.AlphaString()
	fieldGens["Name"] = gen.AlphaString()
	fieldGens["AppID"] = gen.AlphaString()
	fieldGens["Image"] = gen.AlphaString()
	fieldGens["Config"] = configGenerator()
	fieldGens["ResourceConfig"] = resourceConfigGenerator(t)
	fieldGens["Annotations"] = annotationGenerator()
	fieldGens["CreatedAt"] = datetimeGenerator()
	fieldGens["UpdatedAt"] = datetimeGenerator()

	fnFieldCount := fnReflectType().NumField()

	if fnFieldCount != len(fieldGens) {
		t.Fatalf("Fn struct field count, %d, does not match fn generator field count, %d", fnFieldCount, len(fieldGens))
	}

	return fieldGens
}

func fnGenerator(t *testing.T) gopter.Gen {
	return gen.Struct(fnReflectType(), fnFieldGenerators(t))
}

func TestFnEquality(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("A fn should always equal itself", prop.ForAll(
		func(fn Fn) bool {
			return fn.Equals(&fn)
		},
		fnGenerator(t),
	))

	properties.Property("A fn should always equal a clone of itself", prop.ForAll(
		func(fn Fn) bool {
			clone := fn.Clone()
			return fn.Equals(clone)
		},
		fnGenerator(t),
	))

	fnFieldGens := fnFieldGenerators(t)

	properties.Property("A fn should never match a modified version of itself", prop.ForAll(
		func(fn Fn) bool {
			for fieldName, fieldGen := range fnFieldGens {

				if fieldName == "CreatedAt" ||
					fieldName == "UpdatedAt" {
					continue
				}

				currentValue, newValue := novelValue(t, reflect.ValueOf(fn), fieldName, fieldGen)

				clone := fn.Clone()
				s := reflect.ValueOf(clone).Elem()
				field := s.FieldByName(fieldName)

				field.Set(newValue)

				if fn.Equals(clone) {
					t.Errorf("Changed field, %s, from {%v} to {%v}, but still equal.", fieldName, currentValue, newValue)
					return false
				}
			}
			return true
		},
		fnGenerator(t),
	))

	properties.TestingRun(t)
}

func TestValidateFnName(t *testing.T) {
	tooLongName := "7"
	for i := 0; i < MaxLengthFnName+1; i++ {
		tooLongName += "7"
	}

	testCases := []struct {
		Name string
		Want error
	}{
		{"valid_name-101", nil},
		{"unescaped/path", ErrFnsInvalidName},
		{tooLongName, ErrFnsTooLongName},
		{"", ErrFnsMissingName},
	}

	for _, testCase := range testCases {
		fn := Fn{Name: testCase.Name}
		got := fn.ValidateName()

		if got != testCase.Want {
			t.Errorf("Fn.ValidateName() failed for %q - wanted: %q but got: %q",
				testCase.Name, testCase.Want, got)
		}
	}
}

func TestValidateFn(t *testing.T) {
	type test struct {
		Fn   Fn
		Want error
	}

	testCases := []test{
		{generateValidFn(), nil},
	}

	// Generate valid Fn objects then break them to check validation is working
	testFn := generateValidFn()
	testFn.Name = ""
	testCases = append(testCases, test{testFn, ErrFnsMissingName})

	testFn = generateValidFn()
	testFn.Name = "unescaped/path"
	testCases = append(testCases, test{testFn, ErrFnsInvalidName})

	testFn = generateValidFn()
	testFn.AppID = ""
	testCases = append(testCases, test{testFn, ErrFnsMissingAppID})

	testFn = generateValidFn()
	testFn.Image = ""
	testCases = append(testCases, test{testFn, ErrFnsMissingImage})

	testFn = generateValidFn()
	testFn.Timeout = 0
	testCases = append(testCases, test{testFn, ErrFnsInvalidTimeout})

	testFn = generateValidFn()
	testFn.IdleTimeout = 0
	testCases = append(testCases, test{testFn, ErrFnsInvalidIdleTimeout})

	testFn = generateValidFn()
	testFn.Memory = 0
	testCases = append(testCases, test{testFn, ErrInvalidMemory})

	for _, testCase := range testCases {
		got := testCase.Fn.Validate()

		if got != testCase.Want {
			t.Errorf("Fn.Validate() failed for '%+v' - wanted: %q but got: %q",
				testCase.Fn, testCase.Want, got)
		}
	}
}

// Generate an Fn structure which passes validation
func generateValidFn() Fn {
	return Fn{
		Name:  "valid_name",
		AppID: "valid_app",
		Image: "valid_image",
		ResourceConfig: ResourceConfig{
			Timeout:     1,
			IdleTimeout: 1,
			Memory:      1,
		},
	}
}
