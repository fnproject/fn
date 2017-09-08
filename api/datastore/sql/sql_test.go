package sql

import (
	"net/url"
	"os"
	"testing"

	"github.com/fnproject/fn/api/datastore/internal/datastoretest"
	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/models"
)

func TestDatastore(t *testing.T) {
	defer os.RemoveAll("sqlite_test_dir")
	u, err := url.Parse("sqlite3://sqlite_test_dir")
	if err != nil {
		t.Fatal(err)
	}
	f := func() models.Datastore {
		os.RemoveAll("sqlite_test_dir")
		ds, err := New(u)
		if err != nil {
			t.Fatal(err)
		}
		// we don't want to test the validator, really
		return datastoreutil.NewValidator(ds)
	}
	datastoretest.Test(t, f)
}
