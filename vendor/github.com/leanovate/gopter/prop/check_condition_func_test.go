package prop

import (
	"reflect"
	"testing"
)

func TestCheckCondition(t *testing.T) {
	call, err := checkConditionFunc(0, 0)
	if err == nil || call != nil {
		t.Error("Should not work for integers")
	}

	call, err = checkConditionFunc(func(a, b int) bool {
		return false
	}, 1)
	if err == nil || call != nil {
		t.Error("Should not work with wrong number of arguments")
	}

	call, err = checkConditionFunc(func(a, b int) {
	}, 2)
	if err == nil || call != nil {
		t.Error("Should not work witout return")
	}

	call, err = checkConditionFunc(func(a, b int) (int, int, int) {
		return 0, 0, 0
	}, 2)
	if err == nil || call != nil {
		t.Error("Should not work with too many return")
	}

	call, err = checkConditionFunc(func(a, b int) (int, int) {
		return 0, 0
	}, 2)
	if err == nil || call != nil {
		t.Error("Should not work if second return is not an error")
	}

	var calledA, calledB int
	call, err = checkConditionFunc(func(a, b int) bool {
		calledA = a
		calledB = b
		return true
	}, 2)
	if err != nil || call == nil {
		t.Error("Should work")
	}
	result := call([]reflect.Value{
		reflect.ValueOf(123),
		reflect.ValueOf(456),
	})
	if calledA != 123 || calledB != 456 {
		t.Errorf("Invalid parameters: %d, %d", calledA, calledB)
	}
	if !result.Success() {
		t.Errorf("Invalid result: %#v", result)
	}
}
