package migrations

import (
	"context"
	"database/sql"
	"github.com/fnproject/fn/api/common"
	"github.com/pressly/goose"
	"sort"
)

var (
	migrations = goose.Migrations{}
)

// each new migration will add corresponding column to the table def
var initialTables = [...]string{`CREATE TABLE IF NOT EXISTS routes (
	app_name varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	image varchar(256) NOT NULL,
	format varchar(16) NOT NULL,
	memory int NOT NULL,
	timeout int NOT NULL,
	idle_timeout int NOT NULL,
	type varchar(16) NOT NULL,
	headers text NOT NULL,
	config text NOT NULL,
	PRIMARY KEY (app_name, path)
);`,

	`CREATE TABLE IF NOT EXISTS apps (
	name varchar(256) NOT NULL PRIMARY KEY,
	config text NOT NULL
);`,

	`CREATE TABLE IF NOT EXISTS calls (
	created_at varchar(256) NOT NULL,
	started_at varchar(256) NOT NULL,
	completed_at varchar(256) NOT NULL,
	status varchar(256) NOT NULL,
	id varchar(256) NOT NULL,
	app_name varchar(256) NOT NULL,
	path varchar(256) NOT NULL,
	PRIMARY KEY (id)
);`,

	`CREATE TABLE IF NOT EXISTS logs (
	id varchar(256) NOT NULL PRIMARY KEY,
	app_name varchar(256) NOT NULL,
	log text NOT NULL
);`,
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

func getPreviousMigration() (*goose.Migration, error) {
	migrations = sortAndConnectMigrations(migrations)
	last, err := migrations.Last()
	if err != nil {
		return nil, err
	}
	return last, nil
}

func DownByOne(driver string, db *sql.DB) error {
	goose.SetDialect(driver)
	last, err := getPreviousMigration()
	if err != nil {
		return err
	}
	return last.Down(db)
}

func ApplyMigrations(ctx context.Context, driver string, db *sql.DB) error {
	log := common.Logger(ctx)

	for _, v := range initialTables {
		_, err := db.ExecContext(ctx, v)
		if err != nil {
			return err
		}
	}

	goose.SetDialect(driver)
	migrations = sortAndConnectMigrations(migrations)
	current, err := goose.GetDBVersion(db)
	log.Info("current datastore version: ", current)
	if err != nil {
		if err != goose.ErrNoNextVersion {
			return err
		}
	}
	// datastore is fresh new
	if current == -1 {
		current = 0
	}
	// bad migrations?
	if current > int64(len(migrations)) {
		log.Fatal("malformed datastore version ")
	}
	//latest version, nothing to do
	//if current == int64(len(migrations)) {
	//	return nil
	//}
	// we run migrations only in case if there is a new version(s)
	leftToUpgrade := migrations[current:]
	log.Debug("Migrations left to apply: ", len(leftToUpgrade))
	// we can trust this, list is sorted
	for _, m := range leftToUpgrade {
		if err = m.Up(db); err != nil {
			log.Error("migrations upgrade error: ", err.Error())
			return err
		}
	}

	return nil
}
