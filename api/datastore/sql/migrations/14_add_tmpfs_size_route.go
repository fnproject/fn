package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up14(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes ADD tmpfs_size int;")

	return err
}

func down14(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes DROP COLUMN tmpfs_size;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(14),
		UpFunc:      up14,
		DownFunc:    down14,
	})
}
