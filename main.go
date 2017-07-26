package main

import (
	"context"

	"github.com/fnproject/fn/api/server"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)
	// Setup your custom extensions, listeners, etc here
	funcServer.Start(ctx)
}
