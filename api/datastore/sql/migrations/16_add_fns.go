package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up16(ctx context.Context, tx *sqlx.Tx) error {
	createQuery := `CREATE TABLE IF NOT EXISTS fns (
	id varchar(256) NOT NULL PRIMARY KEY,
	name varchar(256) NOT NULL UNIQUE,
	app_id varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	cpus int NOT NULL,
	memory int NOT NULL,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	config text NOT NULL,
	annotations text NOT NULL,
	created_at varchar(256) NOT NULL,
	updated_at varchar(256) NOT NULL,
	CONSTRAINT fk_app_id FOREIGN KEY (app_id) REFERENCES apps(id)
);`
	_, err := tx.ExecContext(ctx, createQuery)
	return err
}

func down16(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE fns;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(16),
		UpFunc:      up16,
		DownFunc:    down16,
	})
}
