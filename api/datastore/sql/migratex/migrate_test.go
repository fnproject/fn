package migratex

import (
	"context"
	"errors"
	"fmt"
	"testing"

	_ "github.com/fnproject/fn/api/datastore/sql/sqlite"
	"github.com/jmoiron/sqlx"
)

const testsqlite3 = "file::memory:?mode=memory&cache=shared"

type tm struct{}

func (t *tm) Up(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS foo (
		bar bigint NOT NULL PRIMARY KEY
	)`)
	return err
}

func (t *tm) Down(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE foo")
	return err
}

func (t *tm) Version() int64 { return 1 }

func TestMigrateUp(t *testing.T) {
	x := new(tm)

	db, err := sqlx.Open("sqlite3", testsqlite3)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	do := func() error {
		tx, err := db.Beginx()
		if err != nil {
			return err
		}

		defer tx.Commit()

		version, dirty, err := Version(ctx, tx)
		if version != NilVersion || err != nil || dirty {
			return fmt.Errorf("version err: %v %v", err, dirty)
		}

		if version != NilVersion {
			return errors.New("found existing version in db, nuke it")
		}

		err = Up(ctx, tx, []Migration{x})
		if err != nil {
			return err
		}

		version, dirty, err = Version(ctx, tx)
		if err != nil || dirty {
			return fmt.Errorf("version err: %v %v", err, dirty)
		}

		if version != x.Version() {
			return errors.New("version did not update, migration should have ran.")
		}
		return nil
	}

	err = do()
	if err != nil {
		t.Fatalf("couldn't run migrations: %v", err)
	}

	do = func() error {
		// make sure the table is there.
		// TODO find a db agnostic way of doing this.
		//	query := db.Rebind(`SELECT foo FROM sqlite_master WHERE type = 'table'`)
		query := db.Rebind(`SELECT name FROM sqlite_master where type='table' AND name='foo'`)
		var result string
		err = db.QueryRowContext(ctx, query).Scan(&result)
		if err != nil {
			return fmt.Errorf("foo check: %v", err)
		}

		if result != "foo" {
			return fmt.Errorf("migration version worked but migration didn't work: %v", result)
		}
		return nil
	}

	err = do()
	if err != nil {
		t.Fatalf("migration check failed: %v", err)
	}
}
