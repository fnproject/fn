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

package ochttp

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// The following client HTTP measures are supported for use in custom views:
var (
	ClientRequests, _         = stats.Int64("opencensus.io/http/client/requests", "Number of HTTP requests started", stats.UnitNone)
	ClientRequestBodySize, _  = stats.Int64("opencensus.io/http/client/request_size", "HTTP request body size if set as ContentLength (uncompressed)", stats.UnitBytes)
	ClientResponseBodySize, _ = stats.Int64("opencensus.io/http/client/response_size", "HTTP response body size (uncompressed)", stats.UnitBytes)
	ClientLatency, _          = stats.Float64("opencensus.io/http/client/latency", "End-to-end latency", stats.UnitMilliseconds)
)

// The following tags are applied to stats recorded by this package. Host, Path
// and Method are applied to all measures. StatusCode is not applied to
// ClientRequests, since it is recorded before the status is known.
var (
	// Host is the value of the HTTP Host header.
	Host, _ = tag.NewKey("http.host")
	// StatusCode is the numeric HTTP response status code,
	// or "error" if a transport error occurred and no status code was read.
	StatusCode, _ = tag.NewKey("http.status")
	// Path is the URL path (not including query string) in the request.
	Path, _ = tag.NewKey("http.path")
	// Method is the HTTP method of the request, capitalized (GET, POST, etc.).
	Method, _ = tag.NewKey("http.method")
)

var (
	DefaultSizeDistribution    = view.DistributionAggregation([]float64{0, 1024, 2048, 4096, 16384, 65536, 262144, 1048576, 4194304, 16777216, 67108864, 268435456, 1073741824, 4294967296})
	DefaultLatencyDistribution = view.DistributionAggregation([]float64{0, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000})
)

// Package ochttp provides some convenience views.
// You need to subscribe to the views for data to actually be collected.
var (
	ClientRequestCount, _                 = view.New("opencensus.io/http/client/requests", "Count of HTTP requests started", nil, ClientRequests, view.CountAggregation{})
	ClientRequestBodySizeDistribution, _  = view.New("opencensus.io/http/client/request_size", "Size distribution of HTTP request body", nil, ClientRequestBodySize, DefaultSizeDistribution)
	ClientResponseBodySizeDistribution, _ = view.New("opencensus.io/http/client/response_size", "Size distribution of HTTP response body", nil, ClientResponseBodySize, DefaultSizeDistribution)
	ClientLatencyDistribution, _          = view.New("opencensus.io/http/client/latency", "Latency distribution of HTTP requests", nil, ClientLatency, DefaultLatencyDistribution)

	ClientRequestCountByMethod, _ = view.New(
		"opencensus.io/http/client/request_count_by_method",
		"Client request count by HTTP method",
		[]tag.Key{Method},
		ClientRequests,
		view.CountAggregation{})
	ClientResponseCountByStatusCode, _ = view.New(
		"opencensus.io/http/client/response_count_by_status_code",
		"Client response count by status code",
		[]tag.Key{StatusCode},
		ClientLatency,
		view.CountAggregation{})

	DefaultViews = []*view.View{
		ClientRequestCount,
		ClientRequestBodySizeDistribution,
		ClientResponseBodySizeDistribution,
		ClientLatencyDistribution,
		ClientRequestCountByMethod,
		ClientResponseCountByStatusCode,
	}
)
