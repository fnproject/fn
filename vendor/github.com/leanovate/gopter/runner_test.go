package gopter

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRunnerSingleWorker(t *testing.T) {
	parameters := DefaultTestParameters()
	testRunner := &runner{
		parameters: parameters,
		worker: func(num int, shouldStop shouldStop) *TestResult {
			return &TestResult{
				Status:    TestPassed,
				Succeeded: 1,
				Discarded: 0,
			}
		},
	}

	result := testRunner.runWorkers()

	if result.Status != TestPassed ||
		result.Succeeded != 1 ||
		result.Discarded != 0 {
		t.Errorf("Invalid result: %#v", result)
	}
}

func TestRunnerParallelWorkers(t *testing.T) {
	parameters := DefaultTestParameters()
	specs := []struct {
		workers int
		res     []TestResult
		exp     *TestResult
		wait    []int
	}{
		// Test all pass
		{
			workers: 50,
			res: []TestResult{
				{
					Status:    TestPassed,
					Succeeded: 10,
					Discarded: 1,
				},
			},
			exp: &TestResult{
				Status:    TestPassed,
				Succeeded: 500,
				Discarded: 50,
			},
		},
		// Test exhausted
		{
			workers: 50,
			res: []TestResult{
				{
					Status:    TestExhausted,
					Succeeded: 1,
					Discarded: 10,
				},
			},
			exp: &TestResult{
				Status:    TestExhausted,
				Succeeded: 50,
				Discarded: 500,
			},
		},
		// Test all fail
		{
			workers: 50,
			res: []TestResult{
				{
					Status:    TestFailed,
					Succeeded: 0,
					Discarded: 0,
					Labels:    []string{"some label"},
					Error:     errors.New("invalid result 0 != 1"),
				},
			},
			exp: &TestResult{
				Status:    TestFailed,
				Succeeded: 0,
				Discarded: 0,
				Labels:    []string{"some label"},
				Error:     errors.New("invalid result 0 != 1"),
			},
		},
		// a pass and failure
		{
			workers: 2,
			res: []TestResult{
				{
					Status:    TestPassed,
					Succeeded: 94,
					Discarded: 1,
				},
				{
					Status:    TestFailed,
					Succeeded: 4,
					Discarded: 3,
					Labels:    []string{"some label"},
					Error:     errors.New("invalid result 0 != 2"),
				},
			},
			exp: &TestResult{
				Status:    TestFailed,
				Succeeded: 98,
				Discarded: 4,
				Labels:    []string{"some label"},
				Error:     errors.New("invalid result 0 != 2"),
			},
			wait: []int{1, 0},
		},
		// a pass and multiple failures (first failure returned)
		{
			workers: 3,
			res: []TestResult{
				{
					Status:    TestPassed,
					Succeeded: 94,
					Discarded: 1,
				},
				{
					Status:    TestFailed,
					Succeeded: 3,
					Discarded: 2,
					Labels:    []string{"worker 1"},
					Error:     errors.New("worker 1 error"),
				},
				{
					Status:    TestFailed,
					Succeeded: 1,
					Discarded: 1,
					Labels:    []string{"worker 2"},
					Error:     errors.New("worker 2 error"),
				},
			},
			exp: &TestResult{
				Status:    TestFailed,
				Succeeded: 98,
				Discarded: 4,
				Labels:    []string{"worker 1"},
				Error:     errors.New("worker 1 error"),
			},
			wait: []int{0, 1, 2},
		},
	}

	for specIdx, spec := range specs {
		parameters.Workers = spec.workers

		testRunner := &runner{
			parameters: parameters,
			worker: func(num int, shouldStop shouldStop) *TestResult {
				if num < len(spec.wait) {
					time.Sleep(time.Duration(spec.wait[num]) * time.Second)
				}

				if num < len(spec.res) {
					return &spec.res[num]
				}

				return &spec.res[0]
			},
		}

		result := testRunner.runWorkers()

		if result.Time <= 0 {
			t.Errorf("[%d] expected result time to be positive number but got %s", specIdx, result.Time)
		}

		// This is not deterministic and
		// have validated above the time
		result.Time = 0

		if !reflect.DeepEqual(result, spec.exp) {
			t.Errorf("[%d] expected test result %#v but got %#v",
				specIdx,
				spec.exp,
				result,
			)
		}
	}
}
