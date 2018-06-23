package sql

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/fnproject/fn/api/datastore/internal/datastoretest"
	"github.com/fnproject/fn/api/datastore/internal/datastoreutil"
	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/fnproject/fn/api/datastore/sql/migrations"
	_ "github.com/fnproject/fn/api/datastore/sql/mysql"
	_ "github.com/fnproject/fn/api/datastore/sql/postgres"
	_ "github.com/fnproject/fn/api/datastore/sql/sqlite"
	logstoretest "github.com/fnproject/fn/api/logs/testing"
	"github.com/fnproject/fn/api/models"
	"github.com/jmoiron/sqlx"
)

// since New with fresh dbs skips all migrations:
// * open a fresh db on latest version
// * run all down migrations
// * run all up migrations
// [ then run tests against that db ]
func newWithMigrations(ctx context.Context, url *url.URL) (*SQLStore, error) {
	ds, err := newDS(ctx, url)
	if err != nil {
		return nil, err
	}

	err = ds.Tx(func(tx *sqlx.Tx) error {
		return migratex.Down(ctx, tx, migrations.Migrations)
	})
	if err != nil {
		return nil, err
	}

	// go through New, to ensure our Up logic works in there...
	ds, err = newDS(ctx, url)
	if err != nil {
		return nil, err
	}

	return ds, nil
}

func TestDatastore(t *testing.T) {
	ctx := context.Background()
	defer os.RemoveAll("sqlite_test_dir")
	u, err := url.Parse("sqlite3://sqlite_test_dir")
	if err != nil {
		t.Fatal(err)
	}
	f := func(t *testing.T) *SQLStore {
		os.RemoveAll("sqlite_test_dir")
		ds, err := newDS(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		// we don't want to test the validator, really
		return ds
	}
	f2 := func(t *testing.T) models.Datastore {
		ds := f(t)
		return datastoreutil.NewValidator(ds)
	}
	datastoretest.RunAllTests(t, f2, datastoretest.NewBasicResourceProvider())

	// also logs
	logstoretest.Test(t, f(t))

	// NOTE: sqlite3 does not like ALTER TABLE DROP COLUMN so do not run
	// migration tests against it, only pg and mysql -- should prove UP migrations
	// will likely work for sqlite3, but may need separate testing by devs :(

	// if being run from test script (CI) poke around for pg and mysql containers
	// to run tests against them too. this runs with a fresh db first run, then
	// will down migrate all migrations, up migrate, and run tests again.

	both := func(u *url.URL) {
		f := func(t *testing.T) *SQLStore {
			ds, err := newDS(ctx, u)
			if err != nil {
				t.Fatal(err)
			}
			ds.clear()
			if err != nil {
				t.Fatal(err)
			}
			return ds
		}
		f2 := func(t *testing.T) models.Datastore {
			ds := f(t)
			return datastoreutil.NewValidator(ds)
		}

		// test fresh w/o migrations
		datastoretest.RunAllTests(t, f2, datastoretest.NewBasicResourceProvider())

		// also test sql implements logstore
		logstoretest.Test(t, f(t))

		f = func(t *testing.T) *SQLStore {
			t.Log("with migrations now!")
			ds, err := newWithMigrations(ctx, u)
			if err != nil {
				t.Fatal(err)
			}
			ds.clear()
			if err != nil {
				t.Fatal(err)
			}
			return ds
		}
		f2 = func(t *testing.T) models.Datastore {
			ds := f(t)
			return datastoreutil.NewValidator(ds)
		}

		// test that migrations work & things work with them
		datastoretest.RunAllTests(t, f2, datastoretest.NewBasicResourceProvider())

		// also test sql implements logstore
		logstoretest.Test(t, f(t))
	}

	if pg := os.Getenv("POSTGRES_URL"); pg != "" {
		u, err := url.Parse(pg)
		if err != nil {
			t.Fatal(err)
		}

		both(u)
	}

	if mysql := os.Getenv("MYSQL_URL"); mysql != "" {
		u, err := url.Parse(mysql)
		if err != nil {
			t.Fatal(err)
		}

		both(u)
	}

}

func TestClose(t *testing.T) {
	ctx := context.Background()
	defer os.RemoveAll("sqlite_test_dir")
	u, err := url.Parse("sqlite3://sqlite_test_dir")
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll("sqlite_test_dir")
	ds, err := newDS(ctx, u)
	if err != nil {
		t.Fatal(err)
	}

	if err := ds.Close(); err != nil {
		t.Fatalf("Failed to close datastore: %v", err)
	}
}
