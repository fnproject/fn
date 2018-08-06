package arbitrary_test

import (
	"errors"
	"math/cmplx"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/arbitrary"
	"github.com/leanovate/gopter/gen"
)

type QudraticEquation struct {
	A, B, C complex128
}

func (q *QudraticEquation) Eval(x complex128) complex128 {
	return q.A*x*x + q.B*x + q.C
}

func (q *QudraticEquation) Solve() (complex128, complex128, error) {
	if q.A == 0 {
		return 0, 0, errors.New("No solution")
	}
	v := q.B*q.B - 4*q.A*q.C
	v = cmplx.Sqrt(v)
	return (-q.B + v) / 2 / q.A, (-q.B - v) / 2 / q.A, nil
}

func Example_quadratic() {
	parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice

	arbitraries := arbitrary.DefaultArbitraries()
	arbitraries.RegisterGen(gen.Complex128Box(-1e8-1e8i, 1e8+1e8i)) // Only use complex values within a range

	properties := gopter.NewProperties(parameters)

	properties.Property("Quadratic equations can be solved (as pointer)", arbitraries.ForAll(
		func(quadratic *QudraticEquation) bool {
			x1, x2, err := quadratic.Solve()
			if err != nil {
				return true
			}

			return cmplx.Abs(quadratic.Eval(x1)) < 1e-5 && cmplx.Abs(quadratic.Eval(x2)) < 1e-5
		}))

	properties.Property("Quadratic equations can be solved (as struct)", arbitraries.ForAll(
		func(quadratic QudraticEquation) bool {
			x1, x2, err := quadratic.Solve()
			if err != nil {
				return true
			}

			return cmplx.Abs(quadratic.Eval(x1)) < 1e-5 && cmplx.Abs(quadratic.Eval(x2)) < 1e-5
		}))

	properties.Property("Quadratic equations can be solved alternative", arbitraries.ForAll(
		func(a, b, c complex128) bool {
			quadratic := &QudraticEquation{
				A: a,
				B: b,
				C: c,
			}
			x1, x2, err := quadratic.Solve()
			if err != nil {
				return true
			}

			return cmplx.Abs(quadratic.Eval(x1)) < 1e-5 && cmplx.Abs(quadratic.Eval(x2)) < 1e-5
		}))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// + Quadratic equations can be solved (as pointer): OK, passed 100 tests.
	// + Quadratic equations can be solved (as struct): OK, passed 100 tests.
	// + Quadratic equations can be solved alternative: OK, passed 100 tests.
}
