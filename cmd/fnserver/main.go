package main

import (
	"context"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/drivers/docker"
	"github.com/fnproject/fn/api/logs/s3"
	"github.com/fnproject/fn/api/server"
	// EXTENSIONS: Add extension imports here or use `fn build-server`. Learn more: https://github.com/fnproject/fn/blob/master/docs/operating/extending.md
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
	agent.RegisterAgentViews(keys)
	agent.RegisterDockerViews(keys)
	agent.RegisterContainerViews(keys)

	// Register docker client views
	docker.RegisterViews(keys)

	// Register s3 log views
	s3.RegisterViews(keys)
}
