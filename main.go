package main

import (
	"context"

	"github.com/fnproject/fn/api/server"
	"github.com/fnproject/fn/api/completer"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)

	completer.SetupFromEnv(ctx,funcServer)
	// Setup your custom extensions, listeners, etc here
	funcServer.Start(ctx)
}
