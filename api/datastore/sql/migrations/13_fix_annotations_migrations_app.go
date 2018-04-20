package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up12(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps MODIFY annotations TEXT NULLABLE;")

	return err
}

func down12(ctx context.Context, tx *sqlx.Tx) error {
	// annotations became not-null by accident in #10 we don't undo this here.
	return nil
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(12),
		UpFunc:      up12,
		DownFunc:    down12,
	})
}
