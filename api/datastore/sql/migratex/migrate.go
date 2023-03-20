package migratex

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/crc32"
	"sort"
	"strings"

	"github.com/fnproject/fn/api/datastore/sql/dbhelper"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

var (
	// use same migration table name as mattes/migrate, so that we don't have to
	// migrate that.
	// TODO doesn't have to be a glob
	MigrationsTable = "schema_migrations"

	ErrLocked = errors.New("database is locked")
)

func migrateErr(version int64, up bool, err error) ErrMigration {
	dir := "up"
	if !up {
		dir = "down"
		version++
	}
	return ErrMigration(fmt.Sprintf("error running migration. version: %v direction: %v err: %v", version, dir, err))
}

// ErrMigration represents an error running a specific migration in a specific direction
type ErrMigration string

func (e ErrMigration) Error() string { return string(e) }

func dirtyErr(version int64) ErrDirty {
	return ErrDirty(fmt.Sprintf("database is dirty. version: %v", version))
}

// ErrDirty is an error that is returned when a db is dirty.
type ErrDirty string

func (e ErrDirty) Error() string { return string(e) }

const (
	NilVersion = -1
)

type Migration interface {
	Up(context.Context, *sqlx.Tx) error
	Down(context.Context, *sqlx.Tx) error
	Version() int64
}

type sorted []Migration

func (s sorted) Len() int           { return len(s) }
func (s sorted) Less(i, j int) bool { return s[i].Version() < s[j].Version() }
func (s sorted) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

var _ Migration = new(MigFields)

// MigFields implements Migration and can be used for convenience.
type MigFields struct {
	UpFunc      func(context.Context, *sqlx.Tx) error
	DownFunc    func(context.Context, *sqlx.Tx) error
	VersionFunc func() int64
}

func (m MigFields) Up(ctx context.Context, tx *sqlx.Tx) error   { return m.UpFunc(ctx, tx) }
func (m MigFields) Down(ctx context.Context, tx *sqlx.Tx) error { return m.DownFunc(ctx, tx) }
func (m MigFields) Version() int64                              { return m.VersionFunc() }

// TODO instance must have `multiStatements` set to true ?

func Up(ctx context.Context, tx *sqlx.Tx, migs []Migration) error {
	return migrate(ctx, tx, migs, true)
}

func Down(ctx context.Context, tx *sqlx.Tx, migs []Migration) error {
	return migrate(ctx, tx, migs, false)
}

func migrate(ctx context.Context, tx *sqlx.Tx, migs []Migration, up bool) error {
	curVersion, dirty, err := Version(ctx, tx)
	if dirty {
		return dirtyErr(curVersion)
	}
	if err != nil {
		return err
	}

	// TODO we could grab the lock here and hold it over all the migrations,
	// in methodology we want each migration to run in its own tx envelope
	// so that we can make as much progress as possible if we hit an error.
	// not sure it makes much difference either way where we lock.

	if up {
		sort.Sort(sorted(migs))
	} else {
		sort.Sort(sort.Reverse(sorted(migs)))
	}
	for _, m := range migs {
		// skip over migrations we have run
		mVersion := m.Version()
		if (up && curVersion < mVersion) || (!up && curVersion >= mVersion) {

			// do each individually, for large migrations it's better to checkpoint
			// than to try to do them all in one big go.
			// XXX(reed): we could more gracefully handle concurrent databases trying to
			// run migrations here by handling error and feeding back the version.
			// get something working mode for now...
			err := run(ctx, tx, m, up)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func withLock(ctx context.Context, tx *sqlx.Tx, f func(*sqlx.Tx) error) error {
	err := lock(ctx, tx)
	if err != nil {
		return err
	}
	err = f(tx)

	// NOTE: migrations happen on init and if they fail we should close our session with the db
	// which will release the lock, thus, we don't need to futz with the context here to unlock.
	errU := unlock(ctx, tx)

	if errU != nil {
		if err == nil {
			err = errU
		} else {
			err = multiError(err, errU)
		}
	}
	return err
}

var _ error = multiError()

// MultiError holds multiple errors. If you have to handle one of these... I am so sorry.
type MultiError struct {
	Errs []error
}

func multiError(errs ...error) MultiError {
	compactErrs := make([]error, 0)
	for _, e := range errs {
		if e != nil {
			compactErrs = append(compactErrs, e)
		}
	}
	return MultiError{compactErrs}
}

func (m MultiError) Error() string {
	var strs = make([]string, 0)
	for _, e := range m.Errs {
		strs = append(strs, e.Error())
	}
	return strings.Join(strs, "\n")
}

func run(ctx context.Context, tx *sqlx.Tx, m Migration, up bool) error {
	return withLock(ctx, tx, func(tx *sqlx.Tx) error {
		// within the transaction, we need to check the version and ensure this
		// migration has not already been applied.
		curVersion, dirty, err := Version(ctx, tx)
		if dirty {
			return dirtyErr(curVersion)
		}

		// enforce monotonicity
		if up && curVersion != NilVersion && m.Version() != curVersion+1 {
			return fmt.Errorf("non-contiguous migration attempted up: %v != %v", m.Version(), curVersion+1)
		} else if !up && m.Version() != curVersion { // down is always unraveling
			return fmt.Errorf("non-contiguous migration attempted down: %v != %v", m.Version(), curVersion)
		}

		version := m.Version()
		if !up {
			version = m.Version() - 1
		}

		// TODO we don't need the dirty bit anymore since we're using transactions?
		err = SetVersion(ctx, tx, version, true)

		if up {
			err = m.Up(ctx, tx)
		} else {
			err = m.Down(ctx, tx)
		}

		if err != nil {
			return migrateErr(version, up, err)
		}

		err = SetVersion(ctx, tx, version, false)
		return err
	})
}

const advisoryLockIdSalt uint = 1486364155

// inspired by rails migrations, see https://goo.gl/8o9bCT
// NOTE that this means if the db server has multiple databases that use this
// library then this can cause contention... it seems a far cry.
func generateAdvisoryLockId(name string) string {
	sum := crc32.ChecksumIEEE([]byte(name))
	sum = sum * uint32(advisoryLockIdSalt)
	return fmt.Sprintf("%v", sum)
}

func lock(ctx context.Context, tx *sqlx.Tx) error {
	aid := generateAdvisoryLockId(MigrationsTable)

	// pg has special locking & sqlite3 needs no locking
	var query string
	switch tx.DriverName() {
	case "postgres", "pgx", "pq-timeouts", "cloudsqlpostgres":
		query = `SELECT pg_try_advisory_lock(?)`
	case "mysql", "oci8", "ora", "goracle":
		query = "SELECT GET_LOCK(?, -1)"
	case "sqlite3":
		// sqlite3 doesn't have a lock. as long as migrate isn't called concurrently it'll be ok
		return nil
	default:
		return fmt.Errorf("unsupported database, please add this or fix: %v", tx.DriverName())
	}

	query = tx.Rebind(query)

	var success bool
	if err := tx.QueryRowContext(ctx, query, aid).Scan(&success); err != nil {
		return err
	}

	if success {
		return nil
	}

	return ErrLocked
}

func unlock(ctx context.Context, tx *sqlx.Tx) error {
	aid := generateAdvisoryLockId(MigrationsTable)

	var query string
	switch tx.DriverName() {
	case "postgres", "pgx", "pq-timeouts", "cloudsqlpostgres":
		query = `SELECT pg_advisory_unlock(?)`
	case "mysql", "oci8", "ora", "goracle":
		query = `SELECT RELEASE_LOCK(?)`
	case "sqlite3":
		// sqlite3 doesn't have a lock. as long as migrate isn't called concurrently it'll be ok
		return nil
	default:
		return fmt.Errorf("unsupported database, please add this or fix: %v", tx.DriverName())
	}

	query = tx.Rebind(query)

	_, err := tx.ExecContext(ctx, query, aid)
	return err
}

func SetVersion(ctx context.Context, tx *sqlx.Tx, version int64, dirty bool) error {
	err := ensureVersionTable(ctx, tx)
	if err != nil {
		logrus.WithError(err).Error("error ensuring version table")
		return err
	}

	// TODO need to handle down migration better
	// ideally, we have a record of each up/down migration with a timestamp for auditing,
	// this just nukes the whole table which is kinda lame.
	/* #nosec */
	query := tx.Rebind("DELETE FROM " + MigrationsTable)
	if _, err := tx.Exec(query); err != nil {
		logrus.WithError(err).Error("error deleting version table")
		return err
	}

	if version >= 0 {
		/* #nosec */
		query = tx.Rebind(`INSERT INTO ` + MigrationsTable + ` (version, dirty) VALUES (?, ?)`)
		if _, err := tx.ExecContext(ctx, query, version, dirty); err != nil {
			logrus.WithError(err).Error("error updating version table")
			return err
		}
	}

	return nil
}

func Version(ctx context.Context, tx *sqlx.Tx) (version int64, dirty bool, err error) {
	helper, ok := dbhelper.GetHelper(tx.DriverName())
	if !ok {
		return 0, false, fmt.Errorf("no db helper registered for for %s", tx.DriverName())
	}

	tableExists, err := helper.CheckTableExists(tx, MigrationsTable)

	if err != nil {
		return 0, false, err
	}

	if !tableExists {
		return NilVersion, false, nil
	}

	/* #nosec */
	query := tx.Rebind(`SELECT version, dirty FROM ` + MigrationsTable + ` LIMIT 1`)

	err = tx.QueryRowContext(ctx, query).Scan(&version, &dirty)
	switch {
	case err == sql.ErrNoRows:
		return NilVersion, false, nil

	case err != nil:
		return 0, false, err

	default:
		return version, dirty, nil
	}
}

func ensureVersionTable(ctx context.Context, tx *sqlx.Tx) error {
	// TODO it would sure be nice to have timestamps for auditing
	// TODO sqlite3 uses uint64 type? ugha, test.
	query := tx.Rebind(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %v (
		version bigint NOT NULL PRIMARY KEY,
		dirty boolean NOT NULL
	)`, MigrationsTable))
	_, err := tx.ExecContext(ctx, query)
	return err
}
