// Package dbhelper wraps SQL and specific capabilities of an SQL db
package dbhelper

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"net/url"
)

var sqlHelpers []Helper

// Register registers a new SQL helper
func Register(helper Helper) {
	sqlHelpers = append(sqlHelpers, helper)
}

// Helper provides DB-specific SQL capabilities
type Helper interface {
	fmt.Stringer
	// Supports indicates if this helper supports this driver name
	Supports(driverName string) bool
	// PreConnect calculates the connect URL for the db from a canonical URL used in Fn config
	PreConnect(url *url.URL) (string, error)
	// PostCreate  Apply any configuration to the DB prior to use
	PostCreate(db *sqlx.DB) (*sqlx.DB, error)
	// CheckTableExists checks if a table exists in the DB
	CheckTableExists(tx *sqlx.Tx, table string) (bool, error)
	// IsDuplicateKeyError determines if an error indicates if the prior error was caused by a duplicate key insert
	IsDuplicateKeyError(err error) bool
}

// GetHelper returns a helper for a specific driver
func GetHelper(driverName string) (Helper, bool) {
	for _, helper := range sqlHelpers {
		if helper.Supports(driverName) {
			return helper, true
		}
		logrus.Debugf("%s does not support %s", helper, driverName)
	}
	return nil, false
}
