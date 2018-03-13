package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up1(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes ADD created_at text;")
	return err
}

func down1(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE routes DROP COLUMN created_at;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(1),
		UpFunc:      up1,
		DownFunc:    down1,
	})
}
