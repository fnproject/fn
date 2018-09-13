package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up19(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE logs ADD fn_id varchar(256);")

	switch tx.DriverName() {
	case "mysql":
		_, err := tx.ExecContext(ctx, "ALTER TABLE logs MODIFY app_id varchar(256) NULL;")
		return err
	case "postgres", "pgx":
		_, err = tx.ExecContext(ctx, "ALTER TABLE logs ALTER COLUMN app_id DROP NOT NULL;")
		return err
	}

	return err
}

func down19(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE logs DROP COLUMN fn_id;")

	switch tx.DriverName() {
	case "mysql":
		_, err := tx.ExecContext(ctx, "ALTER TABLE logs MODIFY app_id varchar(256) NOT NULL;")
		return err
	case "postgres", "pgx":
		_, err = tx.ExecContext(ctx, "ALTER TABLE logs ALTER COLUMN app_id SET NOT NULL;")
		return err
	}

	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(19),
		UpFunc:      up19,
		DownFunc:    down19,
	})
}
