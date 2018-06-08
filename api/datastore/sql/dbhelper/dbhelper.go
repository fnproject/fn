package dbhelper

import (
	"github.com/jmoiron/sqlx"
	"net/url"
)

var sqlDrivers []SqlHelper

//Add registers a new SQL helper
func Add(driver SqlHelper) {
	sqlDrivers = append(sqlDrivers, driver)
}

type SqlHelper interface {
	Supports(driverName string) bool
	PreInit(url *url.URL) (string, error)
	PostCreate(db *sqlx.DB) (*sqlx.DB, error)
	CheckTableExists(tx *sqlx.Tx, table string) (bool, error)
	IsDuplicateKeyError(err error) bool
}

func GetHelper(driverName string) (SqlHelper, bool) {
	for _, helper := range sqlDrivers {
		if helper.Supports(driverName) {
			return helper, true
		}
	}
	return nil, false
}
