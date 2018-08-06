package gen_test

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestPtrOf(t *testing.T) {
	genParams := gopter.DefaultGenParameters()
	elementGen := gen.Const("element")
	ptrGen := gen.PtrOf(elementGen)

	for i := 0; i < 100; i++ {
		sample, ok := ptrGen(genParams).Retrieve()

		if !ok {
			t.Error("Sample was not ok")
		}
		if sample == nil {
			continue
		}
		stringPtr, ok := sample.(*string)
		if !ok {
			t.Errorf("Sample not pointer to string: %#v", sample)
		} else if *stringPtr != "element" {
			t.Errorf("Sample contains invalid value: %#v %#v", sample, *stringPtr)
		}
	}
}

type Foo string

func TestPtrOfFoo(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	properties := gopter.NewProperties(parameters)

	properties.Property("PtrOf", prop.ForAll(
		func(foo *Foo,
		) bool {
			return true
		},
		gen.PtrOf(GenFoo()),
	))
	properties.TestingRun(t)
}

func GenFoo() gopter.Gen {
	return gen.SliceOfN(16, gen.Rune()).Map(func(v []rune) Foo {
		return Foo(v)
	})
}
