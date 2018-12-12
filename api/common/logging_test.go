package common

import (
	"testing"
)

func TestNormalizeToLowercaseAndUnderscores(t *testing.T) {
	expectedOutput := "foo_bar_baz"
	actual := NormalizeLogField("FooBarBaz")

	if actual != expectedOutput {
		t.Errorf("Error in normalizing string to be in lowercase and underscore format. \n")
		t.Errorf("Expected output: %s \n Actual output: %s \n", expectedOutput, actual)
	}

	expectedOutput = "foo_foo"
	actual = NormalizeLogField("FooFoo")

	if actual != expectedOutput {
		t.Errorf("Error in normalizing string to be in lowercase and underscore format. \n")
		t.Errorf("Expected output: %s \n Actual output: %s \n", expectedOutput, actual)
	}

	expectedOutput = "trigger_id"
	actual = NormalizeLogField("triggerId")

	if actual != expectedOutput {
		t.Errorf("Error in normalizing string to be in lowercase and underscore format. \n")
		t.Errorf("Expected output: %s \n Actual output: %s \n", expectedOutput, actual)
	}

	expectedOutput = "trigger_id"
	actual = NormalizeLogField("triggerID")

	if actual != expectedOutput {
		t.Errorf("Error in normalizing string to be in lowercase and underscore format. \n")
		t.Errorf("Expected output: %s \n Actual output: %s \n", expectedOutput, actual)
	}

	expectedOutput = "async_hwmark"
	actual = NormalizeLogField("asyncHWMark")

	if actual != expectedOutput {
		t.Errorf("Error in normalizing string to be in lowercase and underscore format. \n")
		t.Errorf("Expected output: %s \n Actual output: %s \n", expectedOutput, actual)
	}

	expectedOutput = "foo0_bar"
	actual = NormalizeLogField("foo0Bar")

	if actual != expectedOutput {
		t.Errorf("Error in normalizing string to be in lowercase and underscore format. \n")
		t.Errorf("Expected output: %s \n Actual output: %s \n", expectedOutput, actual)
	}

}
