package migrations

import (
	"context"
	"database/sql"
	"github.com/fnproject/fn/api/models"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up25(ctx context.Context, tx *sqlx.Tx) error {

	// Firstly add a new column shape
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps ADD shape TEXT;")
	if err != nil {
		return err
	}

	rows, err := tx.QueryxContext(ctx, "SELECT id FROM apps;")
	if err != nil {
		return err
	}

	res := []*models.App{}
	for rows.Next() {

		var app models.App
		err := rows.StructScan(&app)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		// Set shape default value as empty string in order to avoid sql scan errors
		app.Shape = ""
		res = append(res, &app)
	}
	err = rows.Close()
	if err != nil {
		return err
	}

	if err := rows.Err(); err != nil {
		return err
	}

	for _, app := range res {
		query := tx.Rebind(`UPDATE apps SET shape=:shape WHERE id=:id`)
		_, err = tx.NamedExecContext(ctx, query, app)
		if err != nil {
			return err
		}
	}
	return nil
}


func down25(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE apps DROP COLUMN shape;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(25),
		UpFunc:      up25,
		DownFunc:    down25,
	})
}
