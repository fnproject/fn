package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up18(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE calls ADD fn_id varchar(256);")

	switch tx.DriverName() {
	case "mysql":
		_, err := tx.ExecContext(ctx, "ALTER TABLE calls MODIFY app_id varchar(256) NULL;")
		return err
	case "postgres", "pgx":
		_, err = tx.ExecContext(ctx, "ALTER TABLE calls ALTER COLUMN app_id DROP NOT NULL;")
		return err
	}

	return err
}

func down18(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE calls DROP COLUMN fn_id;")

	switch tx.DriverName() {
	case "mysql":
		_, err := tx.ExecContext(ctx, "ALTER TABLE calls MODIFY app_id varchar(256) NOT NULL;")
		return err
	case "postgres", "pgx":
		_, err = tx.ExecContext(ctx, "ALTER TABLE calls ALTER COLUMN app_id SET NOT NULL;")
		return err
	}

	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(18),
		UpFunc:      up18,
		DownFunc:    down18,
	})
}
