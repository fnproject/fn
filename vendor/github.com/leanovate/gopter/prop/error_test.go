package prop_test

import (
	"errors"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"
)

func TestErrorProp(t *testing.T) {
	p := prop.ErrorProp(errors.New("Booom"))
	result := p(gopter.DefaultGenParameters())

	if result.Status != gopter.PropError || result.Error == nil || result.Error.Error() != "Booom" {
		t.Errorf("Invalid error prop result: %#v", result)
	}
}
