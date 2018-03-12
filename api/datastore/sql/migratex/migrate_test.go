package migratex

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const testsqlite3 = "file::memory:?mode=memory&cache=shared"

type tm struct{}

func (t *tm) Up(tx *sqlx.Tx) error {
	_, err := tx.Exec(`CREATE TABLE IF NOT EXISTS foo (
		bar bigint NOT NULL PRIMARY KEY
	)`)
	return err
}

func (t *tm) Down(tx *sqlx.Tx) error {
	_, err := tx.Exec("DROP TABLE foo")
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

	err = tx(ctx, db, func(tx *sqlx.Tx) error {
		version, dirty, err := Version(ctx, tx)
		if version != NilVersion || err != nil || dirty {
			return fmt.Errorf("version err: %v %v", err, dirty)
		}

		if version != NilVersion {
			return errors.New("found existing version in db, nuke it")
		}

		err = Up(ctx, db, []Migration{x})
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
	})

	if err != nil {
		t.Fatalf("bad things happened: %v", err)
	}
}
