package prop_test

import (
	"strings"
	"unicode"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func MisimplementedConcat(a, b string) string {
	if strings.IndexFunc(a, unicode.IsDigit) > 5 {
		return b
	}
	return a + b
}

// Example_invalidconcat demonstrates shrinking of string
// Kudos to @exarkun and @itamarst for finding this issue
func Example_invalidconcat() {
	parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice

	properties := gopter.NewProperties(parameters)

	properties.Property("length is sum of lengths", prop.ForAll(
		func(a, b string) bool {
			return MisimplementedConcat(a, b) == a+b
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	// When using testing.T you might just use: properties.TestingRun(t)
	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// ! length is sum of lengths: Falsified after 17 passed tests.
	// ARG_0: bahbxh6
	// ARG_0_ORIGINAL (2 shrinks): pkpbahbxh6
	// ARG_1: l
	// ARG_1_ORIGINAL (1 shrinks): dl
}
