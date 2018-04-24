package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up12(ctx context.Context, tx *sqlx.Tx) error {

	// clear out any old null values
	// on mysql this may result in some in-flight changes being missed and an error on the alter table below
	_, err := tx.ExecContext(ctx, `UPDATE apps set annotations=(CASE WHEN annotations IS NULL THEN '' ELSE annotations END);`)
	if err != nil {
		return err
	}

	switch tx.DriverName() {

	case "mysql":
		// this implicitly commits but its the last command so should be safe.
		_, err := tx.ExecContext(ctx, "ALTER TABLE apps MODIFY annotations TEXT NOT NULL;")
		return err
	case "postgres", "pgx":
		_, err := tx.ExecContext(ctx, "ALTER TABLE apps ALTER COLUMN annotations DROP NOT NULL;")
		return err
	default: // nuclear option, replace the table using sqlite safe DDL
		_, err := tx.ExecContext(ctx, "ALTER TABLE apps RENAME TO old_apps;")

		if err != nil {
			return err
		}

		newTable := `CREATE TABLE apps (
	id varchar(256),
	name varchar(256) NOT NULL PRIMARY KEY,
	config text NOT NULL,
	annotations text NOT NULL,
	created_at varchar(256),
	updated_at varchar(256)
);`
		_, err = tx.ExecContext(ctx, newTable)
		if err != nil {
			return err
		}
		insertQuery := `INSERT INTO apps(id,name,config,annotations,created_at,updated_at) 
	  					SELECT  id,name,config,annotations,created_at,updated_at FROM old_apps;`

		_, err = tx.ExecContext(ctx, insertQuery)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, "DROP TABLE old_apps;")
		if err != nil {
			return err
		}

		return err

	}

}

func down12(ctx context.Context, tx *sqlx.Tx) error {
	// annotations are in an indeterminate state so we leave this change as it is
	return nil
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(12),
		UpFunc:      up12,
		DownFunc:    down12,
	})
}
