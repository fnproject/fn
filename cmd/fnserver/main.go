package main

import (
	"context"

	"github.com/fnproject/fn/api/server"
	// The trace package is imported in several places by different dependencies and if we don't import explicity here it is
	// initialized every time it is imported and that creates a panic at run time as we register multiple time the handler for
	// /debug/requests. For example see: https://github.com/GoogleCloudPlatform/google-cloud-go/issues/663 and https://github.com/bradleyfalzon/gopherci/issues/101
	_ "golang.org/x/net/trace"
	// EXTENSIONS: Add extension imports here or use `fn build-server`. Learn more: https://github.com/fnproject/fn/blob/master/docs/operating/extending.md
)

func main() {
	ctx := context.Background()
	funcServer := server.NewFromEnv(ctx)
	funcServer.Start(ctx)
}
