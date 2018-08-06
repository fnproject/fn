package prop_test

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestForAllNoShrink(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	simpleForAll := prop.ForAllNoShrink1(
		gen.Const("const value"),
		func(value interface{}) (interface{}, error) {
			return value.(string) == "const value", nil
		},
	)

	simpleResult := simpleForAll.Check(parameters)

	if simpleResult.Status != gopter.TestPassed || simpleResult.Succeeded != parameters.MinSuccessfulTests {
		t.Errorf("Invalid simpleResult: %#v", simpleResult)
	}

	simpleForAllFail := prop.ForAllNoShrink1(
		gen.Const("const value"),
		func(value interface{}) (interface{}, error) {
			return value.(string) != "const value", nil
		},
	)

	simpleResultFail := simpleForAllFail.Check(parameters)

	if simpleResultFail.Status != gopter.TestFailed || simpleResultFail.Succeeded != 0 {
		t.Errorf("Invalid simpleResultFail: %#v", simpleResultFail)
	}

	fail := prop.ForAllNoShrink(0)
	result := fail(gopter.DefaultGenParameters())
	if result.Status != gopter.PropError {
		t.Errorf("Invalid result: %#v", result)
	}

	undecided := prop.ForAllNoShrink(func(a int) bool {
		return true
	}, gen.Int().SuchThat(func(interface{}) bool {
		return false
	}))
	result = undecided(gopter.DefaultGenParameters())
	if result.Status != gopter.PropUndecided {
		t.Errorf("Invalid result: %#v", result)
	}
}
