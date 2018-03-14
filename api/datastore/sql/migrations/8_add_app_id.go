package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/fnproject/fn/api/datastore/sql/migratex"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/jmoiron/sqlx"
)

func up8(ctx context.Context, tx *sqlx.Tx) error {
	addAppIDStatements := []string{
		"ALTER TABLE apps ADD id VARCHAR(256);",
		"ALTER TABLE calls ADD app_id VARCHAR(256);",
		"ALTER TABLE logs ADD app_id VARCHAR(256);",
		"ALTER TABLE routes ADD app_id VARCHAR(256);",
	}
	for _, statement := range addAppIDStatements {
		_, err := tx.ExecContext(ctx, statement)
		return err
	}

	rows, err := tx.QueryxContext(ctx, "SELECT DISTINCT name FROM apps;")
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {

		var app models.App
		err := rows.StructScan(&app)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}

		app.ID = id.New().String()
		query := tx.Rebind(`UPDATE apps SET id=:id WHERE name=:name`)
		_, err = tx.NamedExecContext(ctx, query, app)
		if err != nil {
			return err
		}

		for _, t := range []string{"routes", "calls", "logs"} {
			q := fmt.Sprintf(`UPDATE %v SET app_id=? WHERE app_name=?;`, t)
			_, err = tx.ExecContext(ctx, tx.Rebind(q), app.ID, app.Name)
			if err != nil {
				return err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	dropAppNameStatements := []string{
		"ALTER TABLE routes DROP COLUMN app_name;",
		"ALTER TABLE calls DROP COLUMN app_name;",
		"ALTER TABLE logs DROP COLUMN app_name;",
	}
	for _, statement := range dropAppNameStatements {
		_, err := tx.ExecContext(ctx, statement)
		return err
	}
	return nil

}

func down8(ctx context.Context, tx *sqlx.Tx) error {

	addAppNameStatements := []string{
		"ALTER TABLE calls ADD app_name VARCHAR(256);",
		"ALTER TABLE logs ADD app_name VARCHAR(256);",
		"ALTER TABLE routes ADD app_name VARCHAR(256);",
	}
	for _, statement := range addAppNameStatements {
		_, err := tx.ExecContext(ctx, statement)
		return err
	}

	rows, err := tx.QueryxContext(ctx, "SELECT DISTINCT id, name FROM apps;")
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var app models.App
		err := rows.StructScan(&app)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}

		for _, t := range []string{"routes", "calls", "logs"} {
			q := fmt.Sprintf(`UPDATE %v SET app_name=? WHERE app_id=?;`, t)
			_, err = tx.ExecContext(ctx, tx.Rebind(q), app.Name, app.ID)
			if err != nil {
				return err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	removeAppIDStatemets := []string{
		"ALTER TABLE logs DROP COLUMN app_id;",
		"ALTER TABLE calls DROP COLUMN app_id;",
		"ALTER TABLE routes DROP COLUMN app_id;",
		"ALTER TABLE apps DROP COLUMN id;",
	}
	for _, statement := range removeAppIDStatemets {
		_, err := tx.ExecContext(ctx, statement)
		return err
	}
	return nil
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(8),
		UpFunc:      up8,
		DownFunc:    down8,
	})
}
