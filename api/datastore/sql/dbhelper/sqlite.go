package dbhelper

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	"github.com/prometheus/common/log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type sqliteDriver int

const theSqliteDriver = sqliteDriver(0)

func (sqliteDriver) Supports(scheme string) bool {
	log.Debugf("Checking %s", scheme)
	switch scheme {
	case "sqlite3":
	case "sqlite":
		return true
	}
	return false
}

func (sqliteDriver) PreInit(url *url.URL) (string, error) {
	// make all the dirs so we can make the file..
	dir := filepath.Dir(url.Path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(url.String(), url.Scheme+"://"), nil
}

func (sqliteDriver) PostCreate(db *sqlx.DB) (*sqlx.DB, error) {
	db.SetMaxOpenConns(1)
	return db, nil

}
func (sqliteDriver) CheckTableExists(tx *sqlx.Tx, table string) (bool, error) {
	query := tx.Rebind(fmt.Sprintf(`SELECT count(*)
		FROM sqlite_master
		WHERE name = '%s'
		`, table))

	row := tx.QueryRow(query)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	exists := count > 0
	return exists, nil
}

func (sqliteDriver) IsDuplicateKeyError(err error) bool {
	sqliteErr, ok := err.(sqlite3.Error)
	if ok {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
			return true
		}
	}
	return false
}

func init() {
	Add(theSqliteDriver)
}
