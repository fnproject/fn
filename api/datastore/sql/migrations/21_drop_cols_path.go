package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up21(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE calls DROP COLUMN path;")

	return err
}

func down21(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.Exec("ALTER TABLE calls ADD path varchar(256);")

	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(21),
		UpFunc:      up21,
		DownFunc:    down21,
	})
}
