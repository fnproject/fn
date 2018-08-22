package gopter_test

import (
	"testing"

	"github.com/leanovate/gopter"
)

func TestFlag(t *testing.T) {
	flag := &gopter.Flag{}
	if flag.Get() {
		t.Errorf("Flag should be initially unset: %#v", flag)
	}
	flag.Set()
	if !flag.Get() {
		t.Errorf("Flag should be set: %#v", flag)
	}
	flag.Unset()
	if flag.Get() {
		t.Errorf("Flag should be unset: %#v", flag)
	}
}
