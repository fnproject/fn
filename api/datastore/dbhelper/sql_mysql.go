// +build !ds_sql_off,!ds_sql_some ds_sql_mysql

package dbhelper

import (
	_ "github.com/go-sql-driver/mysql"
	"net/url"
	"github.com/jmoiron/sqlx"
	"github.com/go-sql-driver/mysql"
)

type mysqlDriver int

const theMysqlDriver = mysqlDriver(0)

func (mysqlDriver) PreInit(url *url.URL) (string, error) {
	return url.String(), nil
}

func (mysqlDriver) PostCreate(db *sqlx.DB) (*sqlx.DB, error) {
	return db, nil

}
func (mysqlDriver) CheckTableExists(tx *sqlx.Tx, table string) (bool, error) {
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

func (mysqlDriver) IsDuplicateKeyError(err error) bool {
	switch err.(type) {
	case *mysql.MySQLError:
		if err.Number == 1062 {
			return true
		}
	}
	return false
}

func init() {
	RegisterSqlDriver("mysql", theMysqlDriver)
}
