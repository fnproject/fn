package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/fnproject/fn/api/common"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose"
)

var oldMigrationsTable = "schema_migrations"

func checkOldMigrationTableVersionIfExists(db *sqlx.DB) (version int64, dirty bool, err error) {
	ctx := context.Background()

	q := db.QueryRowContext(
		ctx, "SELECT version, dirty FROM "+oldMigrationsTable+" LIMIT 1")
	q.Scan(&version, &dirty)
	if err == sql.ErrNoRows {
		return -1, false, nil
	} else if err != nil {
		return -1, false, err
	}
	return version, dirty, nil
}

func checkOldMigration(ctx context.Context, db *sqlx.DB) (int64, error) {
	log := common.Logger(ctx)
	current, dirty, err := checkOldMigrationTableVersionIfExists(db)
	if err != nil {
		return -1, err
	}
	if dirty {
		log.Fatal("database corrupted")
	}
	if current > 0 {
		log.Debug("old migration table version is: ", current)
		// will cause partial upgrade, starting from
		// the latest version in the old migration table
		return current, nil
	}
	// will cause full upgrade
	return -1, nil
}

func syncToGooseTable(ctx context.Context, db *sqlx.DB) (goose.Migrations, error) {
	log := common.Logger(ctx)
	goose.SetDialect(db.DriverName())

	// this will create goose migrations table if it doesn't exist
	gooseCurrent, err := goose.GetDBVersion(db.DB)
	if err != nil {
		return nil, err
	}
	log.Debug("current goose migrations version: ", gooseCurrent)

	// will return the latest version
	migrateCurrent, err := checkOldMigration(ctx, db)
	if err != nil {
		return nil, err
	}

	if migrateCurrent > 0 {
		newMigrationsQuery := fmt.Sprintf("INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, ?);")

		WithTx(db, func(tx *sqlx.Tx) error {
			for version := int64(1); version < migrateCurrent+1; version++ {
				_, err = tx.ExecContext(ctx, tx.Rebind(newMigrationsQuery), version, true)
				if err != nil {
					return err
				}
				log.Debug("inserting new goose version: ", version)
			}
			// as soon as we sync two tables we need to drop old one
			_, _ = tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", oldMigrationsTable))
			log.Debug("mattes/migrate table has gone, hooray!")
			return nil
		})
		return sortAndConnectMigrations(migrations)[migrateCurrent:], nil
	}

	// in case of a new datastore this will return a new slice [0:], with all the migrations we have
	// in case of the existing datastore this will return the slice of migrations left to apply from current goose version
	// in case of up-to-date datastore this will return an empty slice because there are no more migrations left to apply
	return sortAndConnectMigrations(migrations)[gooseCurrent:], nil
}
