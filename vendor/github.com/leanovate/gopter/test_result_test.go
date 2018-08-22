package gopter_test

import (
	"testing"

	"github.com/leanovate/gopter"
)

func TestTestResult(t *testing.T) {
	result := &gopter.TestResult{Status: gopter.TestPassed}
	if !result.Passed() {
		t.Errorf("Test not passed: %#v", result)
	}
	if result.Status.String() != "PASSED" {
		t.Errorf("Invalid status: %#v", result)
	}

	result = &gopter.TestResult{Status: gopter.TestProved}
	if !result.Passed() {
		t.Errorf("Test not passed: %#v", result)
	}
	if result.Status.String() != "PROVED" {
		t.Errorf("Invalid status: %#v", result)
	}

	result = &gopter.TestResult{Status: gopter.TestFailed}
	if result.Passed() {
		t.Errorf("Test passed: %#v", result)
	}
	if result.Status.String() != "FAILED" {
		t.Errorf("Invalid status: %#v", result)
	}

	result = &gopter.TestResult{Status: gopter.TestExhausted}
	if result.Passed() {
		t.Errorf("Test passed: %#v", result)
	}
	if result.Status.String() != "EXHAUSTED" {
		t.Errorf("Invalid status: %#v", result)
	}

	result = &gopter.TestResult{Status: gopter.TestError}
	if result.Passed() {
		t.Errorf("Test passed: %#v", result)
	}
	if result.Status.String() != "ERROR" {
		t.Errorf("Invalid status: %#v", result)
	}
}
