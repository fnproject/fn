// Package defaultexts are the extensions that are auto-loaded in to the
// default fnserver binary included here as a package to simplify inclusion in
// testing
package defaultexts

import (
	// import all datastore modules for runtime config
	_ "github.com/fnproject/fn/api/agent/drivers/docker"
	_ "github.com/fnproject/fn/api/datastore/sql"
	_ "github.com/fnproject/fn/api/datastore/sql/mysql"
	_ "github.com/fnproject/fn/api/datastore/sql/postgres"
	_ "github.com/fnproject/fn/api/datastore/sql/sqlite"
)
