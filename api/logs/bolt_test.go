package logs

import (
	"net/url"
	"os"
	"testing"

	"gitlab-odx.oracle.com/odx/functions/api/datastore/bolt"
	logTesting "gitlab-odx.oracle.com/odx/functions/api/logs/testing"
)

const tmpLogDb = "/tmp/func_test_log.db"
const tmpDatastore = "/tmp/func_test_datastore.db"

func TestDatastore(t *testing.T) {
	os.Remove(tmpLogDb)
	os.Remove(tmpDatastore)
	uLog, err := url.Parse("bolt://" + tmpLogDb)
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	uDatastore, err := url.Parse("bolt://" + tmpDatastore)

	fnl, err := NewBolt(uLog)
	if err != nil {
		t.Fatalf("failed to create bolt log datastore: %v", err)
	}
	ds, err := bolt.New(uDatastore)
	if err != nil {
		t.Fatalf("failed to create bolt datastore: %v", err)
	}
	logTesting.Test(t, fnl, ds)
}
