package sqlite

import (
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

func (sqliteHelper) PreConnect(u string) (string, error) {
	// make all the dirs so we can make the file..
	sqliteUrl, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(sqliteUrl.Path)
	err = os.MkdirAll(dir, 0750)
	if err != nil {
		return "", err
	}

	return strings.TrimPrefix(u, "sqlite3://"), nil
}

func (sqliteHelper) PostCreate(db *sqlx.DB) (*sqlx.DB, error) {
	db.SetMaxOpenConns(1)
	return db, nil
}

func (sqliteHelper) CheckTableExists(tx *sqlx.Tx, table string) (bool, error) {
	query := tx.Rebind(`SELECT count(*)
		FROM sqlite_master
		WHERE name = ?`)

	row := tx.QueryRow(query, table)

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
	dbhelper.Register(sqliteHelper(0))
}
