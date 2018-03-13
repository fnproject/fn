package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up6(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps ADD updated_at VARCHAR(256);")
	return err
}

func down6(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps DROP COLUMN updated_at;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(6),
		UpFunc:      up6,
		DownFunc:    down6,
	})
}
