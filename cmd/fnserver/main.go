package main

import (
	"context"

	"github.com/fnproject/fn/api/server"
	// EXTENSIONS: Add extension imports here or use `fn build-server`. Learn more: https://github.com/fnproject/fn/blob/master/docs/operating/extending.md

	_ "github.com/fnproject/fn/api/datastore/sql"
	_ "github.com/fnproject/fn/api/datastore/sql/mysql"
	_ "github.com/fnproject/fn/api/datastore/sql/postgres"
	_ "github.com/fnproject/fn/api/datastore/sql/sqlite"
	_ "github.com/fnproject/fn/api/logs/s3"
	_ "github.com/fnproject/fn/api/mqs/bolt"
	_ "github.com/fnproject/fn/api/mqs/memory"
	_ "github.com/fnproject/fn/api/mqs/redis"
)

func main() {
	ctx := context.Background()
	funcServer := server.NewFromEnv(ctx)
	funcServer.Start(ctx)
}
