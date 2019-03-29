package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up24(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE logs;")
	return err
}

func down24(ctx context.Context, tx *sqlx.Tx) error {
	createQuery := `CREATE TABLE IF NOT EXISTS logs (
	id varchar(256) NOT NULL PRIMARY KEY,
	app_id varchar(256),
	fn_id varchar(256),
	log text NOT NULL
);`

	_, err := tx.ExecContext(ctx, createQuery)
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(24),
		UpFunc:      up24,
		DownFunc:    down24,
	})
}
