package logs

import (
	"net/url"
	"os"
	"testing"

	"context"

	"github.com/fnproject/fn/api/datastore/sql"
	logTesting "github.com/fnproject/fn/api/logs/testing"
)

const tmpLogDb = "/tmp/func_test_log.db"

func TestDatastore(t *testing.T) {
	os.Remove(tmpLogDb)
	ctx := context.Background()
	uLog, err := url.Parse("sqlite3://" + tmpLogDb)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	ds, err := sql.New(ctx, uLog)
	if err != nil {
		t.Fatalf("failed to create sqlite3 datastore: %v", err)
	}
	logTesting.Test(t, ds)
}
