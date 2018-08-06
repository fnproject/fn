package arbitrary_test

import (
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/arbitrary"
)

type MyStringType string
type MyInt8Type int8
type MyInt16Type int16
type MyInt32Type int32
type MyInt64Type int64
type MyUInt8Type uint8
type MyUInt16Type uint16
type MyUInt32Type uint32
type MyUInt64Type uint64

type Foo struct {
	Name MyStringType
	Id1  MyInt8Type
	Id2  MyInt16Type
	Id3  MyInt32Type
	Id4  MyInt64Type
	Id5  MyUInt8Type
	Id6  MyUInt16Type
	Id7  MyUInt32Type
	Id8  MyUInt64Type
}

func Example_arbitrary_structs() {
	parameters := gopter.DefaultTestParametersWithSeed(1234) // Example should generate reproducable results, otherwise DefaultTestParameters() will suffice

	arbitraries := arbitrary.DefaultArbitraries()

	properties := gopter.NewProperties(parameters)

	properties.Property("MyInt64", arbitraries.ForAll(
		func(id MyInt64Type) bool {
			return id > -1000
		}))
	properties.Property("MyUInt32Type", arbitraries.ForAll(
		func(id MyUInt32Type) bool {
			return id < 2000
		}))
	properties.Property("Foo", arbitraries.ForAll(
		func(foo *Foo) bool {
			return true
		}))
	properties.Property("Foo2", arbitraries.ForAll(
		func(foo Foo) bool {
			return true
		}))

	properties.Run(gopter.ConsoleReporter(false))
	// Output:
	// ! MyInt64: Falsified after 6 passed tests.
	// ARG_0: -1000
	// ARG_0_ORIGINAL (54 shrinks): -1601066829744837253
	// ! MyUInt32Type: Falsified after 0 passed tests.
	// ARG_0: 2000
	// ARG_0_ORIGINAL (23 shrinks): 2161922319
	// + Foo: OK, passed 100 tests.
	// + Foo2: OK, passed 100 tests.
}
