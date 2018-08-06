package gopter_test

import (
	"os"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()

	properties := gopter.NewProperties(parameters)

	properties.Property("always fail", prop.ForAll(
		func(v int32) bool {
			return false
		},
		gen.Int32(),
	))

	fakeT := &testing.T{}
	properties.TestingRun(fakeT)
	if !fakeT.Failed() {
		t.Errorf("fakeT has not failed")
	}
}

func TestPropertiesCustomReporter(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	properties := gopter.NewProperties(parameters)

	properties.Property("always fail", prop.ForAll(
		func(v int32) bool {
			return false
		},
		gen.Int32(),
	))

	fakeT := &testing.T{}
	properties.TestingRun(fakeT, gopter.NewFormatedReporter(true, 160, os.Stdout))
	if !fakeT.Failed() {
		t.Errorf("fakeT has not failed")
	}
}
