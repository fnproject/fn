package main

import (
	"context"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/logs/s3"
	"github.com/fnproject/fn/api/server"
	// The trace package is imported in several places by different dependencies and if we don't import explicity here it is
	// initialized every time it is imported and that creates a panic at run time as we register multiple time the handler for
	// /debug/requests. For example see: https://github.com/GoogleCloudPlatform/google-cloud-go/issues/663 and https://github.com/bradleyfalzon/gopherci/issues/101
	_ "golang.org/x/net/trace"
	// EXTENSIONS: Add extension imports here or use `fn build-server`. Learn more: https://github.com/fnproject/fn/blob/master/docs/operating/extending.md

	_ "github.com/fnproject/fn/api/server/defaultexts"
)

func main() {
	ctx := context.Background()
	funcServer := server.NewFromEnv(ctx)

	registerViews()
	funcServer.Start(ctx)
}

func registerViews() {
	// Register views in agent package
	keys := []string{"fn_appname", "fn_path"}
	dist := []float64{1, 10, 50, 100, 250, 500, 1000, 10000, 60000, 120000}

	agent.RegisterAgentViews(keys, dist)
	agent.RegisterDockerViews(keys, dist)
	agent.RegisterContainerViews(keys, dist)

	// Register docker client views
	docker.RegisterViews(keys, dist)

	// Register s3 log views
	s3.RegisterViews(keys, dist)

	apiKeys := []string{"path", "method", "status"}
	apiDist := []float64{0, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000}

	server.RegisterAPIViews(apiKeys, apiDist)
}
