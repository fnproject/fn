package agent

import (
	"testing"

	"github.com/fnproject/fn/api/models"
)

func TestValidateFnInvokeTypeDefault(t *testing.T) {
	expected := models.TypeSync
	actual := validateFnInvokeType("")
	if actual != expected {
		t.Errorf("Expected call type set to '%s' but got '%s'", expected, actual)
	}
}

func TestValidateFnInvokeTypeUnknowData(t *testing.T) {
	expected := models.TypeSync
	actual := validateFnInvokeType("meta foo type")
	if actual != expected {
		t.Errorf("Expected call type set to '%s' but got '%s'", expected, actual)
	}
}

func TestValidateFnInvokeTypeDeteached(t *testing.T) {
	expected := models.TypeDetached
	actual := validateFnInvokeType("detached")
	if actual != expected {
		t.Errorf("Expected call type set to '%s' but got '%s'", expected, actual)
	}
}
