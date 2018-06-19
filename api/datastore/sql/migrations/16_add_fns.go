package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up16(ctx context.Context, tx *sqlx.Tx) error {
	createQuery := `CREATE TABLE IF NOT EXISTS fns (
	id varchar(256) NOT NULL,
	name varchar(256) NOT NULL,
	app_id varchar(256) NOT NULL,
	app_name varchar(256) NOT NULL,
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
  PRIMARY KEY (app_name, name),
	CONSTRAINT fk_app_name FOREIGN KEY (app_name) REFERENCES apps(name),
  CONSTRAINT name_app_id_unique UNIQUE (app_id, name)
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
