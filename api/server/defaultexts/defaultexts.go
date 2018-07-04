// Package defaultexts are the extensions that are auto-loaded in to the
// default fnserver binary included here as a package to simplify inclusion in
// testing
package defaultexts

import (
	// import all datastore/log/mq modules for runtime config
	_ "github.com/fnproject/fn/api/datastore/sql"
	_ "github.com/fnproject/fn/api/datastore/sql/mysql"
	_ "github.com/fnproject/fn/api/datastore/sql/postgres"
	_ "github.com/fnproject/fn/api/datastore/sql/sqlite"
	_ "github.com/fnproject/fn/api/logs/s3"
	_ "github.com/fnproject/fn/api/mqs/bolt"
	_ "github.com/fnproject/fn/api/mqs/memory"
	_ "github.com/fnproject/fn/api/mqs/redis"
)
