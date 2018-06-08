package dbhelper

import (
	"net/url"
	"github.com/jmoiron/sqlx"
	"sort"
	"strings"
)

var sqlDrivers = make(map[string]SqlDriver)

//RegisterSqlDriver registers a
func RegisterSqlDriver(id string, driver SqlDriver) {
	sqlDrivers[id] = driver
}

type SqlDriver interface {
	PreInit(url *url.URL) (string, error)
	PostCreate(db *sqlx.DB) (*sqlx.DB, error)
	CheckTableExists(tx *sqlx.Tx, table string) (bool, error)
	IsDuplicateKeyError(err error) bool
}

func GetHelper(helper string) (SqlDriver, bool) {
	driver, ok := sqlDrivers[helper]
	return driver, ok
}


func ListHelpers() []string {
	driverList := make([]string, len(sqlDrivers))
	for k := range sqlDrivers {
		driverList = append(driverList, k)
	}
	sort.Slice(driverList, func(i, j int) bool {
		return strings.Compare(driverList[i], driverList[j]) < 0
	})
	return driverList
}