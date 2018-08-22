package gen_test

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestFail(t *testing.T) {
	fail := gen.Fail(reflect.TypeOf(""))

	value, ok := fail.Sample()

	if value != nil || ok {
		t.Fail()
	}
}
