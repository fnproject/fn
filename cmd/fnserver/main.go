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
	registerViews()

	funcServer := server.NewFromEnv(ctx)
	funcServer.Start(ctx)
}

func registerViews() {
	keys := []string{}

	latencyDist := []float64{1, 10, 50, 100, 250, 500, 1000, 10000, 60000, 120000}

	// IO buckets in Mbits (Using same buckets for network + disk)
	mb := float64(131072)
	ioDist := []float64{0, mb, 4 * mb, 8 * mb, 16 * mb, 32 * mb, 64 * mb, 128 * mb, 256 * mb, 512 * mb, 1024 * mb}

	// Memory buckets in MB
	mB := float64(1048576)
	memoryDist := []float64{0, 128 * mB, 256 * mB, 512 * mB, 1024 * mB, 2 * 1024 * mB, 4 * 1024 * mB, 8 * 1024 * mB}

	// 10% granularity buckets
	cpuDist := []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	agent.RegisterRunnerViews(keys, latencyDist)
	agent.RegisterAgentViews(keys, latencyDist)
	agent.RegisterDockerViews(keys, latencyDist, ioDist, ioDist, memoryDist, cpuDist)

	// container views have additional metrics, optional to turn on
	// TODO more cohesive plan for wiring these in
	cKeys := append(keys, agent.AppIDMetricKey.Name(), agent.FnIDMetricKey.Name(), agent.ImageNameMetricKey.Name())
	agent.RegisterContainerViews(cKeys, latencyDist)

	// Register docker client views
	docker.RegisterViews(keys, latencyDist)

	// Register s3 log views
	s3.RegisterViews(keys, latencyDist)

	server.RegisterAPIViews(keys, latencyDist)
}
