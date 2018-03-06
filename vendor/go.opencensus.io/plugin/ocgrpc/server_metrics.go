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
//

package ocgrpc

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// The following variables are measures are recorded by ServerHandler:
var (
	ServerErrorCount, _        = stats.Int64("grpc.io/server/error_count", "RPC Errors", stats.UnitNone)
	ServerServerElapsedTime, _ = stats.Float64("grpc.io/server/server_elapsed_time", "Server elapsed time in msecs", stats.UnitMilliseconds)
	ServerRequestBytes, _      = stats.Int64("grpc.io/server/request_bytes", "Request bytes", stats.UnitBytes)
	ServerResponseBytes, _     = stats.Int64("grpc.io/server/response_bytes", "Response bytes", stats.UnitBytes)
	ServerStartedCount, _      = stats.Int64("grpc.io/server/started_count", "Number of server RPCs (streams) started", stats.UnitNone)
	ServerFinishedCount, _     = stats.Int64("grpc.io/server/finished_count", "Number of server RPCs (streams) finished", stats.UnitNone)
	ServerRequestCount, _      = stats.Int64("grpc.io/server/request_count", "Number of server RPC request messages", stats.UnitNone)
	ServerResponseCount, _     = stats.Int64("grpc.io/server/response_count", "Number of server RPC response messages", stats.UnitNone)
)

// TODO(acetechnologist): This is temporary and will need to be replaced by a
// mechanism to load these defaults from a common repository/config shared by
// all supported languages. Likely a serialized protobuf of these defaults.

// Predefined views may be subscribed to collect data for the above measures.
// As always, you may also define your own custom views over measures collected by this
// package. These are declared as a convenience only; none are subscribed by
// default.
var (
	ServerErrorCountView, _ = view.New(
		"grpc.io/server/error_count",
		"RPC Errors",
		[]tag.Key{KeyMethod, KeyStatus},
		ServerErrorCount,
		view.CountAggregation{})

	ServerServerElapsedTimeView, _ = view.New(
		"grpc.io/server/server_elapsed_time",
		"Server elapsed time in msecs",
		[]tag.Key{KeyMethod},
		ServerServerElapsedTime,
		DefaultMillisecondsDistribution)

	ServerRequestBytesView, _ = view.New(
		"grpc.io/server/request_bytes",
		"Request bytes",
		[]tag.Key{KeyMethod},
		ServerRequestBytes,
		DefaultBytesDistribution)

	ServerResponseBytesView, _ = view.New(
		"grpc.io/server/response_bytes",
		"Response bytes",
		[]tag.Key{KeyMethod},
		ServerResponseBytes,
		DefaultBytesDistribution)

	ServerRequestCountView, _ = view.New(
		"grpc.io/server/request_count",
		"Count of request messages per server RPC",
		[]tag.Key{KeyMethod},
		ServerRequestCount,
		DefaultMessageCountDistribution)

	ServerResponseCountView, _ = view.New(
		"grpc.io/server/response_count",
		"Count of response messages per server RPC",
		[]tag.Key{KeyMethod},
		ServerResponseCount,
		DefaultMessageCountDistribution)
)

// All default server views provided by this package:
var (
	DefaultServerViews = []*view.View{
		ServerErrorCountView,
		ServerServerElapsedTimeView,
		ServerRequestBytesView,
		ServerResponseBytesView,
		ServerRequestCountView,
		ServerResponseCountView,
	}
)

// TODO(jbd): Add roundtrip_latency, uncompressed_request_bytes, uncompressed_response_bytes, request_count, response_count.
