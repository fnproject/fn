package convey_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/arbitrary"
	. "github.com/leanovate/gopter/convey"
	"github.com/leanovate/gopter/gen"
	. "github.com/smartystreets/goconvey/convey"
)

type QudraticEquation struct {
	A, B, C float64
}

func (q *QudraticEquation) Eval(x float64) float64 {
	return q.A*x*x + q.B*x + q.C
}

func (q *QudraticEquation) Solve() (float64, float64, error) {
	if q.A == 0 {
		return 0, 0, errors.New("No solution")
	}
	v := q.B*q.B - 4*q.A*q.C
	if v < 0 {
		return 0, 0, errors.New("No solution")
	}
	v = math.Sqrt(v)
	return (-q.B + v) / 2 / q.A, (-q.B - v) / 2 / q.A, nil
}

func TestShouldSucceedForAll(t *testing.T) {
	Convey("Given a check for quadratic equations", t, func() {
		checkSolve := func(quadratic *QudraticEquation) bool {
			x1, x2, err := quadratic.Solve()
			if err != nil {
				return true
			}

			return math.Abs(quadratic.Eval(x1)) < 1e-5 && math.Abs(quadratic.Eval(x2)) < 1e-5
		}

		Convey("Then check with arbitraries succeeds", func() {
			arbitraries := arbitrary.DefaultArbitraries()
			arbitraries.RegisterGen(gen.Float64Range(-1e5, 1e5))

			So(checkSolve, ShouldSucceedForAll, arbitraries)

			Convey("And test parameters may be modified", func() {
				parameters := gopter.DefaultTestParameters()
				parameters.MinSuccessfulTests = 200

				So(checkSolve, ShouldSucceedForAll, arbitraries, parameters)
			})
		})

		Convey("Then check with explicit generator succeeds", func() {
			anyQudraticEquation := gen.StructPtr(reflect.TypeOf(QudraticEquation{}), map[string]gopter.Gen{
				"A": gen.Float64Range(-1e5, 1e5),
				"B": gen.Float64Range(-1e5, 1e5),
				"C": gen.Float64Range(-1e5, 1e5),
			})

			So(checkSolve, ShouldSucceedForAll, anyQudraticEquation)
		})

		Convey("Expect fail", func() {
			parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice
			result := ShouldSucceedForAll(func(i int) bool {
				return i > 500
			}, gen.Int(), parameters)

			So(result, ShouldStartWith, "! : Falsified after 1 passed tests.\nARG_0: 0\nARG_0_ORIGINAL (1 shrinks): -642623569")
		})
	})
}
