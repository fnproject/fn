package logs

import (
	logTesting "github.com/fnproject/fn/api/logs/testing"
	"testing"
)

func TestDatastore(t *testing.T) {
	ds := logTesting.SetupSQLiteDS(t)
	logTesting.Test(t, ds, ds)
}
