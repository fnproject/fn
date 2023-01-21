package models

import (
	"reflect"
	"testing"
	"time"

	"github.com/fnproject/fn/api/common"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

var stringType = reflect.TypeOf("")
var intType = reflect.TypeOf(0)

func appReflectType() reflect.Type {
	app := App{}
	return reflect.TypeOf(app)
}

func configGenerator() gopter.Gen {
	return gen.MapOf(gen.AlphaString(), gen.AlphaString()).Map(func(m map[string]string) Config {
		return Config(m)
	})
}

func annotationGenerator() gopter.Gen {
	annotation1, _ := EmptyAnnotations().With("anAnnotation1", "value1")
	annotation2, _ := EmptyAnnotations().With("anAnnotation2", "value2")

	return gen.OneConstOf(annotation1, annotation2)
}

func datetimeGenerator() gopter.Gen {
	return gen.Time().Map(func(t time.Time) common.DateTime {
		return common.DateTime(t)
	})
}

func appFieldGenerators(t *testing.T) map[string]gopter.Gen {
	fieldGens := make(map[string]gopter.Gen)
	fieldGens["ID"] = gen.AlphaString()
	fieldGens["Name"] = gen.AlphaString()
	fieldGens["Config"] = configGenerator()
	fieldGens["Annotations"] = annotationGenerator()
	//fieldGens["Architectures"] = gen.SliceOfN(1, gen.OneConstOf("x86", "arm"))
	fieldGens["Architectures"] = gen.SliceOfN(0, gen.OneConstOf(""))
	fieldGens["SyslogURL"] = gen.AlphaString().Map(func(s string) *string {
		return &s
	})
	fieldGens["CreatedAt"] = datetimeGenerator()
	fieldGens["UpdatedAt"] = datetimeGenerator()

	appFieldCount := appReflectType().NumField()

	if appFieldCount != len(fieldGens) {
		t.Fatalf("App struct field count, %d, does not match app generator field count, %d", appFieldCount, len(fieldGens))
	}

	return fieldGens
}

func appGenerator(t *testing.T) gopter.Gen {
	return gen.Struct(appReflectType(), appFieldGenerators(t))
}

func novelValue(t *testing.T, originalInstance reflect.Value, fieldName string, fieldGen gopter.Gen) (interface{}, reflect.Value) {
	newValue, result := fieldGen.Sample()
	if !result {
		t.Fatalf("Error sampling field generator, %s, %v", fieldName, result)
	}

	field := originalInstance.FieldByName(fieldName)
	currentValue := field.Interface()

	for i := 0; i < 100; i++ {
		if fieldName == "Annotations" {
			if !newValue.(Annotations).Equals(currentValue.(Annotations)) {
				break
			}
		} else if fieldName == "Config" {
			if !newValue.(Config).Equals(currentValue.(Config)) {
				break
			}
		} else {
			if newValue != currentValue {
				break
			}
		}
		newValue, result = fieldGen.Sample()
		if !result {
			t.Fatalf("Error sampling field generator, %s, %v", fieldName, result)
		}

		if i == 99 {
			t.Fatalf("Failed to generate a novel value for field, %s", fieldName)
		}
	}
	return currentValue, reflect.ValueOf(newValue)
}

func TestAppEquality(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("An app should always equal itself", prop.ForAll(
		func(app App) bool {
			return app.Equals(&app)
		},
		appGenerator(t),
	))

	properties.Property("An app should always equal a clone of itself", prop.ForAll(
		func(app App) bool {
			clone := app.Clone()
			return app.Equals(clone)
		},
		appGenerator(t),
	))

	appFieldGens := appFieldGenerators(t)

	properties.Property("An app should never match a modified version of itself", prop.ForAll(
		func(app App) bool {
			for fieldName, fieldGen := range appFieldGens {

				if fieldName == "CreatedAt" ||
					fieldName == "UpdatedAt" {
					continue
				}

				currentValue, newValue := novelValue(t, reflect.ValueOf(app), fieldName, fieldGen)

				clone := app.Clone()
				s := reflect.ValueOf(clone).Elem()
				field := s.FieldByName(fieldName)

				field.Set(newValue)

				if app.Equals(clone) {
					t.Errorf("Changed field, %s, from {%v} to {%v}, but still equal.", fieldName, currentValue, newValue)
					return false
				}
			}
			return true
		},
		appGenerator(t),
	))

	properties.TestingRun(t)
}

func TestValidateAppName(t *testing.T) {
	tooLongName := "7"
	for i := 0; i < MaxLengthAppName+1; i++ {
		tooLongName += "7"
	}

	testCases := []struct {
		Name string
		Want error
	}{
		{"valid_name-101", nil},
		{tooLongName, ErrAppsTooLongName},
		{"", ErrMissingName},
		{"invalid.character", ErrAppsInvalidName},
	}

	for _, testCase := range testCases {
		app := App{Name: testCase.Name}
		got := app.ValidateName()

		if got != testCase.Want {
			t.Errorf(
				"App.ValidateName() failed for %q - wanted: %q but got: %q",
				testCase.Name, testCase.Want, got)
		}
	}
}

func TestValidateApp(t *testing.T) {
	valid_name := "valid_name"
	valid_syslog := "tcp://localhost:13371"

	testCases := []struct {
		App  App
		Want error
	}{
		{App{Name: valid_name, SyslogURL: &valid_syslog}, nil},
		{App{Name: ""}, ErrMissingName},
	}

	for _, testCase := range testCases {
		got := testCase.App.Validate()

		if got != testCase.Want {
			t.Errorf("App.Validate() failed for '%+v' - wanted: %q but got: %q",
				testCase.App, testCase.Want, got)
		}
	}
}
