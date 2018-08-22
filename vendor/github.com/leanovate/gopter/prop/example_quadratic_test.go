package prop_test

import (
	"errors"
	"math"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func solveQuadratic(a, b, c float64) (float64, float64, error) {
	if a == 0 {
		return 0, 0, errors.New("No solution")
	}
	v := b*b - 4*a*c
	if v < 0 {
		return 0, 0, errors.New("No solution")
	}
	v = math.Sqrt(v)
	return (-b + v) / 2 / a, (-b - v) / 2 / a, nil
}

func Example_quadratic() {
	parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice

	properties := gopter.NewProperties(parameters)

	properties.Property("solve quadratic", prop.ForAll(
		func(a, b, c float64) bool {
			x1, x2, err := solveQuadratic(a, b, c)
			if err != nil {
				return true
			}
			return math.Abs(a*x1*x1+b*x1+c) < 1e-5 && math.Abs(a*x2*x2+b*x2+c) < 1e-5
		},
		gen.Float64(),
		gen.Float64(),
		gen.Float64(),
	))

	properties.Property("solve quadratic with resonable ranges", prop.ForAll(
		func(a, b, c float64) bool {
			x1, x2, err := solveQuadratic(a, b, c)
			if err != nil {
				return true
			}
			return math.Abs(a*x1*x1+b*x1+c) < 1e-5 && math.Abs(a*x2*x2+b*x2+c) < 1e-5
		},
		gen.Float64Range(-1e8, 1e8),
		gen.Float64Range(-1e8, 1e8),
		gen.Float64Range(-1e8, 1e8),
	))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// ! solve quadratic: Falsified after 0 passed tests.
	// ARG_0: -1.4667384313385178e-05
	// ARG_0_ORIGINAL (187 shrinks): -1.0960555181801604e+51
	// ARG_1: 0
	// ARG_1_ORIGINAL (1 shrinks): -1.1203884793568249e+96
	// ARG_2: 6.481285637227244e+10
	// ARG_2_ORIGINAL (905 shrinks): 1.512647219322138e+281
	// + solve quadratic with resonable ranges: OK, passed 100 tests.
}
