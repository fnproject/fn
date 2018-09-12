package logs

import (
	"testing"

	logTesting "github.com/fnproject/fn/api/logs/testing"
)

func TestMock(t *testing.T) {
	ls := NewMock()
	logTesting.Test(t, ls)
}
