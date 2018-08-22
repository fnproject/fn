package gopter_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
)

func TestNewGenResult(t *testing.T) {
	result := gopter.NewGenResult(123, gopter.NoShrinker)
	value, ok := result.Retrieve()

	if !ok || value != 123 || result.ResultType.Kind() != reflect.Int {
		t.Errorf("Invalid result: %#v", value)
	}
}

func TestNewEmptyResult(t *testing.T) {
	result := gopter.NewEmptyResult(reflect.TypeOf(0))
	value, ok := result.Retrieve()

	if ok || value != nil || result.ResultType.Kind() != reflect.Int {
		t.Errorf("Invalid result: %#v", value)
	}
}
