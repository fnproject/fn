package prop_test

import (
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func Example_timeGen() {
	parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice
	time.Local = time.UTC                                    // Just for this example to generate reproducable results

	properties := gopter.NewProperties(parameters)

	properties.Property("time in range format parsable", prop.ForAll(
		func(actual time.Time) (bool, error) {
			str := actual.Format(time.RFC3339Nano)
			parsed, err := time.Parse(time.RFC3339Nano, str)
			return actual.Equal(parsed), err
		},
		gen.TimeRange(time.Now(), time.Duration(100*24*365)*time.Hour),
	))

	properties.Property("regular time format parsable", prop.ForAll(
		func(actual time.Time) (bool, error) {
			str := actual.Format(time.RFC3339Nano)
			parsed, err := time.Parse(time.RFC3339Nano, str)
			return actual.Equal(parsed), err
		},
		gen.Time(),
	))

	properties.Property("any time format parsable", prop.ForAll(
		func(actual time.Time) (bool, error) {
			str := actual.Format(time.RFC3339Nano)
			parsed, err := time.Parse(time.RFC3339Nano, str)
			return actual.Equal(parsed), err
		},
		gen.AnyTime(),
	))

	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// + time in range format parsable: OK, passed 100 tests.
	// + regular time format parsable: OK, passed 100 tests.
	// ! any time format parsable: Error on property evaluation after 0 passed
	//    tests: parsing time "10000-01-01T00:00:00Z" as
	//    "2006-01-02T15:04:05.999999999Z07:00": cannot parse "0-01-01T00:00:00Z"
	//    as "-"
	// ARG_0: 10000-01-01 00:00:00 +0000 UTC
	// ARG_0_ORIGINAL (45 shrinks): 237903042092-02-10 19:15:18.148265469 +0000
	//    UTC
}
