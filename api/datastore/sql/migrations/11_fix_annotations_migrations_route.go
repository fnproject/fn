package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up11(ctx context.Context, tx *sqlx.Tx) error {

	// clear out any old null values
	// on mysql this may result in some in-flight changes being missed and an error on the alter table below
	_, err := tx.ExecContext(ctx, `UPDATE routes set annotations=(CASE WHEN annotations IS NULL THEN '' ELSE annotations END);`)
	if err != nil {
		return err
	}

	switch tx.DriverName() {

	case "mysql":
		// this implicitly commits but its the last command so should be safe.
		_, err := tx.ExecContext(ctx, "ALTER TABLE routes MODIFY annotations TEXT NOT NULL;")
		return err
	case "postgres", "pgx":
		_, err := tx.ExecContext(ctx, "ALTER TABLE routes ALTER COLUMN annotations DROP NOT NULL;")
		return err
	default:
		_, err := tx.ExecContext(ctx, "ALTER TABLE routes RENAME TO old_routes;")

		if err != nil {
			return err
		}

		newTable := `CREATE TABLE routes (
	app_id varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	memory int NOT NULL,
	cpus int,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	type varchar(16) NOT NULL,
	headers text NOT NULL,
	config text NOT NULL,
	annotations text NOT NULL,
	created_at text,
	updated_at varchar(256),
	PRIMARY KEY (app_id, path)
);`
		_, err = tx.ExecContext(ctx, newTable)
		if err != nil {
			return err
		}
		insertQuery := `INSERT INTO routes(app_id,path,image,format,memory,cpus,timeout,idle_timeout,type,headers,config,annotations,created_at,updated_at) 
	  					SELECT  app_id,path,image,format,memory,cpus,timeout,idle_timeout,type,headers,config,annotations,created_at,updated_at FROM old_routes;`

		_, err = tx.ExecContext(ctx, insertQuery)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, "DROP TABLE old_routes;")
		if err != nil {
			return err
		}

		return err

	}

}

func down11(ctx context.Context, tx *sqlx.Tx) error {
	// annotations are in an indeterminate state so we leave this change as it is
	return nil
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(11),
		UpFunc:      up11,
		DownFunc:    down11,
	})
}
