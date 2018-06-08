// +build !ds_sql_off,!ds_sql_some ds_sql_postgres

package dbhelper

import (
	_ "github.com/go-sql-driver/mysql"
	"net/url"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type postgresDriver int

const thePostgresDriver = postgresDriver(0)

func (postgresDriver) PreInit(url *url.URL) (string, error) {
	return url.String(), nil
}

func (postgresDriver) PostCreate(db *sqlx.DB) (*sqlx.DB, error) {
	return db, nil

}
func (postgresDriver) CheckTableExists(tx *sqlx.Tx, table string) (bool, error) {
	query := tx.Rebind(`SELECT count(*)
	FROM information_schema.TABLES
	WHERE TABLE_NAME = 'apps'
`)

	row := tx.QueryRow(query)

	var count int
	err := row.Scan(&count)
	if err != nil {
		return false, err
	}

	exists := count > 0
	return exists, nil
}

func (postgresDriver) IsDuplicateKeyError(err error) bool {
	switch dbErr:=  err.(type) {
	case *pq.Error:
		if dbErr.Code == "23505" {
			return true
		}
	}
	return false
}

func init() {
	RegisterSqlDriver("postgres", thePostgresDriver)
	RegisterSqlDriver("pgx", thePostgresDriver)

}
