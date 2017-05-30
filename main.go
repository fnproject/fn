package main

import (
	"context"

	"gitlab-odx.oracle.com/odx/functions/api/server"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)
	// Setup your custom extensions, listeners, etc here
	funcServer.Start(ctx)
}
