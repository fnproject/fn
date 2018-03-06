// Copyright 2017, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ocgrpc_test

import (
	"log"

	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
)

func ExampleClientHandler() {
	// Subscribe to collect client request count.
	if err := ocgrpc.ClientRequestCountView.Subscribe(); err != nil {
		log.Fatal(err)
	}

	// Set up a connection to the server with the OpenCensus
	// stats handler to enable stats and tracing.
	conn, err := grpc.Dial("address", grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
}

func ExampleServerHandler() {
	// Subscribe to collect server request count.
	if err := ocgrpc.ServerRequestCountView.Subscribe(); err != nil {
		log.Fatal(err)
	}

	// Set up a new server with the OpenCensus
	// stats handler to enable stats and tracing.
	s := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	_ = s // use s
}
