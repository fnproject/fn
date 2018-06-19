package sqlite

import (
	"fmt"
	"github.com/fnproject/fn/api/datastore/sql/dbhelper"
	"github.com/jmoiron/sqlx"
	"github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type sqliteHelper int

func (sqliteHelper) Supports(scheme string) bool {
	switch scheme {
	case "sqlite3", "sqlite":
		return true
	}
	return false
}

func (sqliteHelper) PreConnect(url *url.URL) (string, error) {
	// make all the dirs so we can make the file..
	dir := filepath.Dir(url.Path)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(url.String(), url.Scheme+"://"), nil
}

func (sqliteHelper) PostCreate(db *sqlx.DB) (*sqlx.DB, error) {
	db.Exec("PRAGMA foreign_keys = ON;")
	db.SetMaxOpenConns(1)
	return db, nil
}

func (sqliteHelper) CheckTableExists(tx *sqlx.Tx, table string) (bool, error) {
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

func (sqliteHelper) String() string {
	return "sqlite"
}

func (sqliteHelper) IsDuplicateKeyError(err error) bool {
	sqliteErr, ok := err.(sqlite3.Error)
	if ok {
		if sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique || sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
			return true
		}
	}
	return false
}

func init() {
	dbhelper.Add(sqliteHelper(0))
}
