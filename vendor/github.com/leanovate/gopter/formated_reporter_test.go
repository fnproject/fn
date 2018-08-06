package gopter

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestConsoleReporter(t *testing.T) {
	var buffer bytes.Buffer
	reporter := &FormatedReporter{
		verbose: false,
		width:   75,
		output:  &buffer,
	}

	reporter.ReportTestResult("test property", &TestResult{Status: TestPassed, Succeeded: 50})
	if buffer.String() != "+ test property: OK, passed 50 tests.\n" {
		t.Errorf("Invalid output: %#v", buffer.String())
	}
	buffer.Reset()

	reporter.ReportTestResult("test property", &TestResult{
		Status:    TestFailed,
		Succeeded: 50,
		Args: PropArgs([]*PropArg{&PropArg{
			Arg: "0",
		}}),
	})
	if buffer.String() != "! test property: Falsified after 50 passed tests.\nARG_0: 0\n" {
		t.Errorf("Invalid output: %#v", buffer.String())
	}
	buffer.Reset()

	reporter.ReportTestResult("test property", &TestResult{
		Status:    TestProved,
		Succeeded: 50,
		Args: PropArgs([]*PropArg{&PropArg{
			Arg:     "0",
			Label:   "somehing",
			OrigArg: "10",
			Shrinks: 6,
		}}),
	})
	if buffer.String() != "+ test property: OK, proved property.\nsomehing: 0\nsomehing_ORIGINAL (6 shrinks): 10\n" {
		t.Errorf("Invalid output: %#v", buffer.String())
	}
	buffer.Reset()

	reporter.ReportTestResult("test property", &TestResult{
		Status:    TestExhausted,
		Succeeded: 50,
		Discarded: 40,
	})
	if buffer.String() != "! test property: Gave up after only 50 passed tests. 40 tests were\n   discarded.\n" {
		t.Errorf("Invalid output: %#v", buffer.String())
	}
	buffer.Reset()

	reporter.ReportTestResult("test property", &TestResult{
		Status:    TestError,
		Error:     errors.New("Poop"),
		Succeeded: 50,
		Args: PropArgs([]*PropArg{&PropArg{
			Arg: "0",
		}}),
	})
	if buffer.String() != "! test property: Error on property evaluation after 50 passed tests: Poop\nARG_0: 0\n" {
		t.Errorf("Invalid output: %#v", buffer.String())
	}
	buffer.Reset()

	reporter.verbose = true
	reporter.ReportTestResult("test property", &TestResult{Status: TestPassed, Succeeded: 50, Time: time.Minute})
	if buffer.String() != "+ test property: OK, passed 50 tests.\nElapsed time: 1m0s\n" {
		t.Errorf("Invalid output: %#v", buffer.String())
	}
	buffer.Reset()
}
