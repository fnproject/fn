package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up15(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps RENAME TO old_apps;")
	if err != nil {
		return err
	}
	newTable := `CREATE TABLE apps (
	id varchar(256) NOT NULL PRIMARY KEY,
	name varchar(256) NOT NULL UNIQUE,
	config text NOT NULL,
	annotations text NOT NULL,
	created_at varchar(256),
	updated_at varchar(256),
	syslog_url text,
	shape text NOT NULL
);`
	_, err = tx.ExecContext(ctx, newTable)
	if err != nil {
		return err
	}
	insertQuery := `INSERT INTO apps(id,name,config,annotations,created_at,updated_at,syslog_url,shape)
					SELECT  id,name,config,annotations,created_at,updated_at,syslog_url,shape FROM old_apps;`

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

func down15(ctx context.Context, tx *sqlx.Tx) error {
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
	updated_at varchar(256),
	syslog_url text,
  	shape text NOT NULL
);`
	_, err = tx.ExecContext(ctx, newTable)
	if err != nil {
		return err
	}
	insertQuery := `INSERT INTO apps(id,name,config,annotations,created_at,updated_at,syslog_url,shape)
					SELECT  id,name,config,annotations,created_at,updated_at,syslog_url,shape FROM old_apps;`

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

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(15),
		UpFunc:      up15,
		DownFunc:    down15,
	})
}
