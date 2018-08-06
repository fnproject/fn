package gen_test

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

func TestRetryUntil(t *testing.T) {
	genParams := gopter.DefaultGenParameters()
	origGen := gen.IntRange(0, 100)
	retryGen := gen.RetryUntil(origGen, func(v int) bool {
		return v > 50
	}, 1000)
	result := retryGen(genParams)
	value, ok := result.Retrieve()
	if value == nil || !ok {
		t.Errorf("RetryGen generated empty result")
	}
	if value.(int) <= 50 {
		t.Errorf("RetryGen generyte invalid value: %#v", value)
	}

	noMatchGen := gen.RetryUntil(origGen, func(v int) bool {
		return v > 500
	}, 100)
	result = noMatchGen(genParams)
	_, ok = result.Retrieve()
	if ok {
		t.Errorf("RetryGen nomatch generated a value")
	}
}
