package gopter_test

import (
	"testing"

	"github.com/leanovate/gopter"
)

func TestPropArg(t *testing.T) {
	gen := constGen("nothing").WithLabel("Label1").WithLabel("Label2")
	prop := gopter.NewPropArg(gen(gopter.DefaultGenParameters()), 1, "nothing", "noth")

	if prop.Label != "Label1, Label2" {
		t.Errorf("Invalid prop.Label: %#v", prop.Label)
	}
	if prop.String() != "nothing" {
		t.Errorf("Invalid prop.Stirng(): %#v", prop.String())
	}
}
