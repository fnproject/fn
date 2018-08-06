package prop_test

import (
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func Example_shrink() {
	parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice

	properties := gopter.NewProperties(parameters)

	properties.Property("fail above 100", prop.ForAll(
		func(arg int64) bool {
			return arg <= 100
		},
		gen.Int64(),
	))

	properties.Property("fail above 100 no shrink", prop.ForAllNoShrink(
		func(arg int64) bool {
			return arg <= 100
		},
		gen.Int64(),
	))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// ! fail above 100: Falsified after 0 passed tests.
	// ARG_0: 101
	// ARG_0_ORIGINAL (56 shrinks): 2041104533947223744
	// ! fail above 100 no shrink: Falsified after 0 passed tests.
	// ARG_0: 6006156956070140861
}
