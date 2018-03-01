package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/fnproject/fn/api/common"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose"
	"sort"
)

var (
	migrations = goose.Migrations{}
)

var tables = [...]string{`CREATE TABLE IF NOT EXISTS routes (
	app_name varchar(256) NOT NULL,
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
	created_at text,
	updated_at varchar(256),
	PRIMARY KEY (app_name, path)
);`,

	`CREATE TABLE IF NOT EXISTS apps (
	name varchar(256) NOT NULL PRIMARY KEY,
	config text NOT NULL,
	created_at varchar(256),
	updated_at varchar(256)
);`,

	`CREATE TABLE IF NOT EXISTS calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_name varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	stats text,
	error text,
	PRIMARY KEY (id)
);`,

	`CREATE TABLE IF NOT EXISTS logs (
	id varchar(256) NOT NULL PRIMARY KEY,
	app_name varchar(256) NOT NULL,
	log text NOT NULL
);`,
}

func WithTx(db *sqlx.DB, f func(*sqlx.Tx) error) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	err = f(tx)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// copy of goose.sortAndConnetMigrations
func sortAndConnectMigrations(migrations goose.Migrations) goose.Migrations {
	sort.Sort(migrations)

	// now that we're sorted in the appropriate direction,
	// populate next and previous for each migration
	for i, m := range migrations {
		prev := int64(-1)
		if i > 0 {
			prev = migrations[i-1].Version
			migrations[i-1].Next = m.Version
		}
		migrations[i].Previous = prev
	}

	return migrations
}

func DownAll(driver string, db *sql.DB) error {
	goose.SetDialect(driver)
	migrations = sortAndConnectMigrations(migrations)

	for {
		currentVersion, err := goose.GetDBVersion(db)
		if err != nil {
			return err
		}

		current, err := migrations.Current(currentVersion)
		if err != nil {
			fmt.Printf("goose: no migrations to run. current version: %d\n", currentVersion)
			return nil
		}

		if current.Version <= 1 {
			fmt.Printf("goose: no migrations to run. current version: %d\n", currentVersion)
			return nil
		}

		if err = current.Down(db); err != nil {
			return err
		}
	}
}

func ApplyMigrations(ctx context.Context, db *sqlx.DB) error {
	goose.SetDialect(db.DriverName())
	log := common.Logger(ctx)

	left, err := syncToGooseTable(ctx, db)
	if err != nil {
		return err
	}
	log.Debug("migrations left to apply: ", len(left))

	for _, m := range left {
		if err := m.Up(db.DB); err != nil {
			return err
		}
	}

	return nil
}
