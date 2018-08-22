package convey

import (
	"bytes"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/arbitrary"
	"github.com/leanovate/gopter/prop"
)

func ShouldSucceedForAll(condition interface{}, params ...interface{}) string {
	var arbitraries *arbitrary.Arbitraries
	parameters := gopter.DefaultTestParameters()
	gens := make([]gopter.Gen, 0)
	for _, param := range params {
		switch param.(type) {
		case *arbitrary.Arbitraries:
			arbitraries = param.(*arbitrary.Arbitraries)
		case *gopter.TestParameters:
			parameters = param.(*gopter.TestParameters)
		case gopter.Gen:
			gens = append(gens, param.(gopter.Gen))
		}
	}

	var property gopter.Prop
	if arbitraries != nil {
		property = arbitraries.ForAll(condition)
	} else {
		property = prop.ForAll(condition, gens...)
	}
	result := property.Check(parameters)

	if !result.Passed() {
		buffer := bytes.NewBufferString("")
		reporter := gopter.NewFormatedReporter(true, 75, buffer)
		reporter.ReportTestResult("", result)

		return buffer.String()
	}
	return ""
}
