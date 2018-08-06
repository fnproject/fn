package gopter_test

import (
	"math"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func Example_sqrt() {
	parameters := gopter.DefaultTestParameters()
	parameters.Rng.Seed(1234) // Just for this example to generate reproducable results

	properties := gopter.NewProperties(parameters)

	properties.Property("greater one of all greater one", prop.ForAll(
		func(v float64) bool {
			return math.Sqrt(v) >= 1
		},
		gen.Float64().SuchThat(func(x float64) bool { return x >= 1.0 }),
	))

	properties.Property("squared is equal to value", prop.ForAll(
		func(v float64) bool {
			r := math.Sqrt(v)
			return math.Abs(r*r-v) < 1e-10*v
		},
		gen.Float64().SuchThat(func(x float64) bool { return x >= 0.0 }),
	))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// + greater one of all greater one: OK, passed 100 tests.
	// + squared is equal to value: OK, passed 100 tests.
}
