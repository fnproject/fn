package gopter_test

import (
	"testing"

	"github.com/leanovate/gopter"
)

func TestPropResult(t *testing.T) {
	result := &gopter.PropResult{Status: gopter.PropProof}
	if !result.Success() || result.Status.String() != "PROOF" {
		t.Errorf("Invalid status: %#v", result)
	}
	other := &gopter.PropResult{Status: gopter.PropTrue}
	if !result.And(other).Success() || result.And(other).Status.String() != "TRUE" {
		t.Errorf("Invalid combined state: %#v", result.And(other))
	}
	if !other.And(result).Success() || other.And(result).Status.String() != "TRUE" {
		t.Errorf("Invalid combined state: %#v", other.And(result))
	}

	result = &gopter.PropResult{Status: gopter.PropTrue}
	if !result.Success() || result.Status.String() != "TRUE" {
		t.Errorf("Invalid status: %#v", result)
	}
	if !result.And(other).Success() || result.And(other).Status.String() != "TRUE" {
		t.Errorf("Invalid combined state: %#v", result.And(other))
	}
	if !other.And(result).Success() || other.And(result).Status.String() != "TRUE" {
		t.Errorf("Invalid combined state: %#v", other.And(result))
	}

	result = &gopter.PropResult{Status: gopter.PropFalse}
	if result.Success() || result.Status.String() != "FALSE" {
		t.Errorf("Invalid status: %#v", result)
	}
	if result.And(other) != result {
		t.Errorf("Invalid combined state: %#v", result.And(other))
	}
	if other.And(result) != result {
		t.Errorf("Invalid combined state: %#v", other.And(result))
	}

	result = &gopter.PropResult{Status: gopter.PropUndecided}
	if result.Success() || result.Status.String() != "UNDECIDED" {
		t.Errorf("Invalid status: %#v", result)
	}
	if result.And(other) != result {
		t.Errorf("Invalid combined state: %#v", result.And(other))
	}
	if other.And(result) != result {
		t.Errorf("Invalid combined state: %#v", other.And(result))
	}

	result = &gopter.PropResult{Status: gopter.PropError}
	if result.Success() || result.Status.String() != "ERROR" {
		t.Errorf("Invalid status: %#v", result)
	}
	if result.And(other) != result {
		t.Errorf("Invalid combined state: %#v", result.And(other))
	}
	if other.And(result) != result {
		t.Errorf("Invalid combined state: %#v", other.And(result))
	}
}

func TestNewPropResult(t *testing.T) {
	trueResult := gopter.NewPropResult(true, "label")
	if trueResult.Status != gopter.PropTrue || trueResult.Labels[0] != "label" {
		t.Errorf("Invalid trueResult: %#v", trueResult)
	}
	falseResult := gopter.NewPropResult(false, "label")
	if falseResult.Status != gopter.PropFalse || falseResult.Labels[0] != "label" {
		t.Errorf("Invalid falseResult: %#v", falseResult)
	}
}
