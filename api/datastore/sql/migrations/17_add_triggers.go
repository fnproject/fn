package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up17(ctx context.Context, tx *sqlx.Tx) error {
	createQuery := `CREATE TABLE IF NOT EXISTS triggers (
	id varchar(256) NOT NULL PRIMARY KEY,
	name varchar(256) NOT NULL,
	app_id varchar(256) NOT NULL,
	fn_id varchar(256) NOT NULL,
	created_at varchar(256) NOT NULL,
	updated_at varchar(256) NOT NULL,
	type varchar(256) NOT NULL,
	source varchar(256) NOT NULL,
	annotations text NOT NULL,
	CONSTRAINT name_app_id_fn_id_unique UNIQUE (app_id, fn_id, name)
);`
	_, err := tx.ExecContext(ctx, createQuery)
	return err
}

func down17(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE triggers;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(17),
		UpFunc:      up17,
		DownFunc:    down17,
	})
}
