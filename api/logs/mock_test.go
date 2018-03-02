package logs

import (
	logTesting "github.com/fnproject/fn/api/logs/testing"
	"testing"
)

func TestMock(t *testing.T) {
	ls := NewMock()
	logTesting.Test(t, ls)
}
