package dbhelper

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"net/url"
)

var sqlHelpers []Helper

//Add registers a new SQL helper
func Add(driver Helper) {
	logrus.Infof("Registering DB helper %s", driver)
	sqlHelpers = append(sqlHelpers, driver)
}

type Helper interface {
	fmt.Stringer
	Supports(driverName string) bool
	PreInit(url *url.URL) (string, error)
	PostCreate(db *sqlx.DB) (*sqlx.DB, error)
	CheckTableExists(tx *sqlx.Tx, table string) (bool, error)
	IsDuplicateKeyError(err error) bool
}

func GetHelper(driverName string) (Helper, bool) {
	for _, helper := range sqlHelpers {
		if helper.Supports(driverName) {
			return helper, true
		}
		logrus.Printf("%s does not support %s", helper, driverName)
	}
	return nil, false
}
