package migrations

import (
	"context"
	"database/sql"
	"github.com/fnproject/fn/api/models"

	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/jmoiron/sqlx"
)

func up26(ctx context.Context, tx *sqlx.Tx) error {

	// Firstly add a new column shape
	_, err := tx.ExecContext(ctx, "ALTER TABLE fns ADD shape TEXT;")
	if err != nil {
		return err
	}

	rows, err := tx.QueryxContext(ctx, "SELECT id,app_id FROM fns;")
	if err != nil {
		return err
	}

	res := []*models.Fn{}
	for rows.Next() {

		var fn models.Fn
		err := rows.StructScan(&fn)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		// Set shape default value as empty string in order to avoid sql scan errors
		fn.Shape = ""
		res = append(res, &fn)
	}
	err = rows.Close()
	if err != nil {
		return err
	}

	if err := rows.Err(); err != nil {
		return err
	}

	for _, app := range res {
		query := tx.Rebind(`UPDATE fns SET shape=:shape WHERE id=:id and app_id=:app_id`)
		_, err = tx.NamedExecContext(ctx, query, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func down26(ctx context.Context, tx *sqlx.Tx) error {
	_, err := tx.ExecContext(ctx, "ALTER TABLE fns DROP COLUMN shape;")
	return err
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(26),
		UpFunc:      up26,
		DownFunc:    down26,
	})
}
