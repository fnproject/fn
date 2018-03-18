package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up8(ctx context.Context, tx *sqlx.Tx) error {
	// Note the DB column name for metadata is "meta_data" to avoid keyword clashes on mysql
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps ADD meta_data TEXT;")

	return err
}

func down8(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps DROP COLUMN meta_data;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(8),
		UpFunc:      up8,
		DownFunc:    down8,
	})
}
