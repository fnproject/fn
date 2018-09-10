package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up18(ctx context.Context, tx *sqlx.Tx) error {
	dropRoutesTable := `DROP TABLE routes;`
	_, err := tx.ExecContext(ctx, dropRoutesTable)
	if err != nil {
		return err
	}

	dropCallsTable := `DROP TABLE calls;`
	_, err = tx.ExecContext(ctx, dropCallsTable)
	if err != nil {
		return err
	}

	createQuery := `CREATE TABLE IF NOT EXISTS calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_id varchar(256) NOT NULL,
	fn_id varchar(256) NOT NULL,
	stats text,
	error text,
	PRIMARY KEY (id)
);`
	_, err = tx.ExecContext(ctx, createQuery)
	return err
}

func down18(ctx context.Context, tx *sqlx.Tx) error {
	return nil
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(18),
		UpFunc:      up18,
		DownFunc:    down18,
	})
}
