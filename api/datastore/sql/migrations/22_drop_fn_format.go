package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up22(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE fns RENAME TO old_fns;")
	if err != nil {
		return err
	}
	var newTable string
	if tx.DriverName() == "postgres" || tx.DriverName() == "pgx" {
		// postgres maintains constraints across table name changes with the old table, so
		// we can't create the constraint again or the tx fails
		newTable = tx.Rebind(`CREATE TABLE IF NOT EXISTS fns (
		id varchar(256) NOT NULL PRIMARY KEY,
		name varchar(256) NOT NULL,
		app_id varchar(256) NOT NULL,
		image varchar(256) NOT NULL,
		memory int NOT NULL,
		timeout int NOT NULL,
		idle_timeout int NOT NULL,
		config text NOT NULL,
		annotations text NOT NULL,
		created_at varchar(256) NOT NULL,
		updated_at varchar(256) NOT NULL
		);`)
	} else {
		newTable = tx.Rebind(`CREATE TABLE IF NOT EXISTS fns (
		id varchar(256) NOT NULL PRIMARY KEY,
		name varchar(256) NOT NULL,
		app_id varchar(256) NOT NULL,
		image varchar(256) NOT NULL,
		memory int NOT NULL,
		timeout int NOT NULL,
		idle_timeout int NOT NULL,
		config text NOT NULL,
		annotations text NOT NULL,
		created_at varchar(256) NOT NULL,
		updated_at varchar(256) NOT NULL,
			CONSTRAINT name_app_id_unique UNIQUE (app_id, name)
		);`)
	}
	_, err = tx.ExecContext(ctx, newTable)
	if err != nil {
		return err
	}
	insertQuery := tx.Rebind(`
	INSERT INTO fns (
		id,
		name,
		app_id,
		image,
		memory,
		timeout,
		idle_timeout,
		config,
		annotations,
		created_at,
		updated_at
	)
	SELECT
		id,
		name,
		app_id,
		image,
		memory,
		timeout,
		idle_timeout,
		config,
		annotations,
		created_at,
		updated_at
	FROM old_fns;
	`)

	_, err = tx.ExecContext(ctx, insertQuery)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP TABLE old_fns;")

	return err
}

func down22(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.Exec("ALTER TABLE fns ADD format varchar(16) NOT NULL;")

	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(22),
		UpFunc:      up22,
		DownFunc:    down22,
	})
}
