package postgres

import (
	"github.com/fnproject/fn/api/datastore/sql/dbhelper"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

type postgresHelper int

func (postgresHelper) Supports(scheme string) bool {
	switch scheme {
	case "postgres", "pgx":
		return true
	}
	return false
}

func (postgresHelper) PreConnect(url string) (string, error) {
	return url, nil
}

func (postgresHelper) PostCreate(db *sqlx.DB) (*sqlx.DB, error) {
	return db, nil

}
func (postgresHelper) CheckTableExists(tx *sqlx.Tx, table string) (bool, error) {
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

func (postgresHelper) String() string {
	return "postgres"
}

func (postgresHelper) IsDuplicateKeyError(err error) bool {
	switch dbErr := err.(type) {
	case *pq.Error:
		if dbErr.Code == "23505" {
			return true
		}
	}
	return false
}

func init() {
	dbhelper.Register(postgresHelper(0))
}
