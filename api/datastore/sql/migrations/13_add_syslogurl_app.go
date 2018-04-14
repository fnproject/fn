package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up13(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps ADD syslog_url TEXT;")

	return err
}

func down13(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps DROP COLUMN syslog_url;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(13),
		UpFunc:      up13,
		DownFunc:    down13,
	})
}
