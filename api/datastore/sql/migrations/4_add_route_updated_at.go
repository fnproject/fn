package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up4(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes ADD updated_at VARCHAR(256);")
	return err
}

func down4(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes DROP COLUMN updated_at;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(4),
		UpFunc:      up4,
		DownFunc:    down4,
	})
}
