package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up20(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE routes;")

	return err
}

func down20(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS routes (
	app_id varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	memory int NOT NULL,
	cpus int,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	tmpfs_size int,
	type varchar(16) NOT NULL,
	headers text NOT NULL,
	config text NOT NULL,
	annotations text NOT NULL,
	created_at text,
	updated_at varchar(256),
	PRIMARY KEY (app_id, path)
);`)

	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(20),
		UpFunc:      up20,
		DownFunc:    down20,
	})
}
