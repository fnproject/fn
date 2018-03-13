// Copyright 2018, OpenCensus Authors
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

package ocgrpc

import (
	"golang.org/x/net/context"

	"google.golang.org/grpc/stats"
)

// ServerHandler implements gRPC stats.Handler recording OpenCensus stats and
// traces. Use with gRPC servers.
type ServerHandler struct {
	// NoTrace may be set to disable recording OpenCensus Spans around
	// gRPC methods.
	NoTrace bool

	// NoStats may be set to disable recording OpenCensus Stats around each
	// gRPC method.
	NoStats bool
}

func (s *ServerHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {
	// no-op
}

func (s *ServerHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	// no-op
	return ctx
}

func (s *ServerHandler) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	if !s.NoTrace {
		s.traceHandleRPC(ctx, rs)
	}
	if !s.NoStats {
		s.statsHandleRPC(ctx, rs)
	}
}

func (s *ServerHandler) TagRPC(ctx context.Context, rti *stats.RPCTagInfo) context.Context {
	if !s.NoTrace {
		ctx = s.traceTagRPC(ctx, rti)
	}
	if !s.NoStats {
		ctx = s.statsTagRPC(ctx, rti)
	}
	return ctx
}
