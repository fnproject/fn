package prop

import (
	"errors"
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
)

func TestConvertResult(t *testing.T) {
	trueResult := convertResult(true, nil)
	if trueResult.Status != gopter.PropTrue || trueResult.Error != nil {
		t.Errorf("Invalid true result: %#v", trueResult)
	}

	falseResult := convertResult(false, nil)
	if falseResult.Status != gopter.PropFalse || falseResult.Error != nil {
		t.Errorf("Invalid false result: %#v", falseResult)
	}

	stringTrueResult := convertResult("", nil)
	if stringTrueResult.Status != gopter.PropTrue ||
		stringTrueResult.Error != nil {
		t.Errorf("Invalid string true result: %#v", stringTrueResult)
	}

	stringFalseResult := convertResult("Something is wrong", nil)
	if stringFalseResult.Status != gopter.PropFalse ||
		stringFalseResult.Error != nil ||
		!reflect.DeepEqual(stringFalseResult.Labels, []string{"Something is wrong"}) {
		t.Errorf("Invalid string false result: %#v", stringFalseResult)
	}

	errorResult := convertResult("Anthing", errors.New("Booom"))
	if errorResult.Status != gopter.PropError || errorResult.Error == nil || errorResult.Error.Error() != "Booom" {
		t.Errorf("Invalid error result: %#v", errorResult)
	}

	propResult := convertResult(&gopter.PropResult{
		Status: gopter.PropProof,
	}, nil)
	if propResult.Status != gopter.PropProof || propResult.Error != nil {
		t.Errorf("Invalid prop result: %#v", propResult)
	}

	invalidResult := convertResult(0, nil)
	if invalidResult.Status != gopter.PropError || invalidResult.Error == nil {
		t.Errorf("Invalid prop result: %#v", invalidResult)
	}
}
