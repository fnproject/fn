package logs

import (
	"net/url"
	"os"
	"testing"

	"gitlab-odx.oracle.com/odx/functions/api/datastore/sql"
	logTesting "gitlab-odx.oracle.com/odx/functions/api/logs/testing"
)

const tmpLogDb = "/tmp/func_test_log.db"

func TestDatastore(t *testing.T) {
	os.Remove(tmpLogDb)
	uLog, err := url.Parse("sqlite3://" + tmpLogDb)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	ds, err := sql.New(uLog)
	if err != nil {
		t.Fatalf("failed to create sqlite3 datastore: %v", err)
	}
	logTesting.Test(t, ds, ds)
}
