package gopter_test

import (
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func spookyCalculation(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	return 2*b + 3*(2+(a+1)+b*(b+1))
}

// Example_labels demonstrates how labels may help, in case of more complex
// conditions.
// The output will be:
//  ! Check spooky: Falsified after 0 passed tests.
//  > Labels of failing property: even result
//  a: 3
//  a_ORIGINAL (44 shrinks): 861384713
//  b: 0
//  b_ORIGINAL (1 shrinks): -642623569
func Example_labels() {
	parameters := gopter.DefaultTestParameters()
	parameters.Rng.Seed(1234) // Just for this example to generate reproducable results
	parameters.MinSuccessfulTests = 10000

	properties := gopter.NewProperties(parameters)

	properties.Property("Check spooky", prop.ForAll(
		func(a, b int) string {
			result := spookyCalculation(a, b)
			if result < 0 {
				return "negative result"
			}
			if result%2 == 0 {
				return "even result"
			}
			return ""
		},
		gen.Int().WithLabel("a"),
		gen.Int().WithLabel("b"),
	))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// ! Check spooky: Falsified after 0 passed tests.
	// > Labels of failing property: even result
	// a: 3
	// a_ORIGINAL (44 shrinks): 861384713
	// b: 0
	// b_ORIGINAL (1 shrinks): -642623569
}
