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

var sqlStatements = [...]string{`CREATE TABLE IF NOT EXISTS routes (
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
	annotations text,
	created_at text,
	updated_at varchar(256),
	PRIMARY KEY (app_id, path)
);`,

	`CREATE TABLE IF NOT EXISTS calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_id varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	stats text,
	error text,
	PRIMARY KEY (id)
);`,

	`CREATE TABLE IF NOT EXISTS logs (
	id varchar(256) NOT NULL PRIMARY KEY,
	app_id varchar(256) NOT NULL,
	log text NOT NULL
);`,
}

var tables = [...]map[string]string{
	{"table": "routes", "statement": sqlStatements[0],
		"columns": "app_id,path,image,format,memory,cpus,timeout," +
			"idle_timeout,type,headers,config,annotations,created_at,updated_at"},

	{"table": "calls", "statement": sqlStatements[1],
		"columns": "created_at,started_at,completed_at,status,id,app_id,path,stats,error"},

	{"table": "logs", "statement": sqlStatements[2],
		"columns": "id,app_id,log"},
}

func up10(ctx context.Context, tx *sqlx.Tx) error {
	addAppIDStatements := []string{
		"ALTER TABLE apps ADD id VARCHAR(256);",
		"ALTER TABLE calls ADD app_id VARCHAR(256);",
		"ALTER TABLE logs ADD app_id VARCHAR(256);",
		"ALTER TABLE routes ADD app_id VARCHAR(256);",
	}
	for _, statement := range addAppIDStatements {
		_, err := tx.ExecContext(ctx, statement)
		if err != nil {
			return err
		}
	}

	rows, err := tx.QueryxContext(ctx, "SELECT DISTINCT name FROM apps;")
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
		app.ID = id.New().String()
		res = append(res, &app)
	}
	err = rows.Close()
	if err != nil {
		return err
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// it is required for some reason, can't do this within the rows iteration.
	for _, app := range res {
		query := tx.Rebind(`UPDATE apps SET id=:id WHERE name=:name`)
		_, err = tx.NamedExecContext(ctx, query, app)
		if err != nil {
			return err
		}

		for _, t := range []string{"routes", "calls", "logs"} {
			q := fmt.Sprintf(`UPDATE %s SET app_id=:id WHERE app_name=:name;`, t)
			_, err = tx.NamedExecContext(ctx, tx.Rebind(q), app)
			if err != nil {
				return err
			}
		}
	}

	if tx.DriverName() != "sqlite3" {
		dropAppNameStatements := []string{
			"ALTER TABLE routes DROP COLUMN app_name;",
			"ALTER TABLE calls DROP COLUMN app_name;",
			"ALTER TABLE logs DROP COLUMN app_name;",
		}
		for _, statement := range dropAppNameStatements {
			_, err := tx.ExecContext(ctx, statement)
			if err != nil {
				return err
			}
		}
	} else {
		for _, t := range tables {
			statement := t["statement"]
			tableName := t["table"]
			columns := t["columns"]
			_, err = tx.ExecContext(ctx, tx.Rebind(fmt.Sprintf("ALTER TABLE %s RENAME TO old_%s;", tableName, tableName)))
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, tx.Rebind(statement))
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, tx.Rebind(fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM old_%s;", tableName, columns, columns, tableName)))
			if err != nil {
				return err
			}

			_, err = tx.ExecContext(ctx, tx.Rebind(fmt.Sprintf("DROP TABLE old_%s;", tableName)))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func down10(ctx context.Context, tx *sqlx.Tx) error {

	addAppNameStatements := []string{
		"ALTER TABLE calls ADD app_name VARCHAR(256);",
		"ALTER TABLE logs ADD app_name VARCHAR(256);",
		"ALTER TABLE routes ADD app_name VARCHAR(256);",
	}
	for _, statement := range addAppNameStatements {
		_, err := tx.ExecContext(ctx, statement)
		if err != nil {
			return err
		}
	}

	rows, err := tx.QueryxContext(ctx, "SELECT DISTINCT id, name FROM apps;")
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
		res = append(res, &app)
	}
	err = rows.Close()
	if err != nil {
		return err
	}

	if err := rows.Err(); err != nil {
		return err
	}

	// it is required for some reason, can't do this within the rows iteration.
	for _, app := range res {
		for _, t := range []string{"routes", "calls", "logs"} {
			q := "UPDATE " + t + " SET app_name=:name WHERE app_id=:id;"
			_, err = tx.NamedExecContext(ctx, tx.Rebind(q), app)
			if err != nil {
				return err
			}
		}
	}

	removeAppIDStatements := []string{
		"ALTER TABLE logs DROP COLUMN app_id;",
		"ALTER TABLE calls DROP COLUMN app_id;",
		"ALTER TABLE routes DROP COLUMN app_id;",
		"ALTER TABLE apps DROP COLUMN id;",
	}
	for _, statement := range removeAppIDStatements {
		_, err := tx.ExecContext(ctx, statement)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	Migrations = append(Migrations, &migratex.MigFields{
		VersionFunc: vfunc(10),
		UpFunc:      up10,
		DownFunc:    down10,
	})
}
