package main

import (
	"context"

	"github.com/fnproject/fn/api/server"
	// EXTENSIONS: Add extension imports here or use `fn build-server`. Learn more: https://github.com/fnproject/fn/blob/master/docs/operating/extending.md

	_ "github.com/fnproject/fn/api/server/defaultexts"
)

func main() {
	ctx := context.Background()
	funcServer := server.NewFromEnv(ctx)
	funcServer.Start(ctx)
}
