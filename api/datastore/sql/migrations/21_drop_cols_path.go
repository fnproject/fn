package migrations

import (
	"context"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up21(ctx context.Context, tx *sqlx.Tx) error {

	_, err := tx.ExecContext(ctx, "ALTER TABLE calls RENAME TO old_calls;")
	if err != nil {
		return err
	}
	newTable := `CREATE TABLE calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_id varchar(256),
	fn_id varchar(256),
	stats text,
	error text,
	PRIMARY KEY (id));`
	_, err = tx.ExecContext(ctx, newTable)
	if err != nil {
		return err
	}
	insertQuery := `INSERT INTO calls(created_at,started_at,completed_at,status,id,app_id,fn_id,stats,error)
					SELECT  created_at,started_at,completed_at,status,id,app_id,fn_id,stats,error FROM old_calls;`

	_, err = tx.ExecContext(ctx, insertQuery)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP TABLE old_calls;")

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
