package server

import (
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	apiRequestCount = stats.Int64("api/request_count", "Number of API requests", stats.UnitDimensionless)
	apiLatency      = stats.Float64("api/latency", "API latency", stats.UnitMilliseconds)
)

var (
	pathKey   = makeKey("path")
	methodKey = makeKey("method")
	statusKey = makeKey("status")
)

var (
	defaultLatencyDistribution = view.Distribution(0, 1, 2, 3, 4, 5, 6, 8, 10, 13, 16, 20, 25, 30, 40, 50, 65, 80, 100, 130, 160, 200, 250, 300, 400, 500, 650, 800, 1000, 2000, 5000, 10000, 20000, 50000, 100000)
)

var (
	apiRequestCountView = &view.View{
		Name:        "api/request_count",
		Description: "Count of API requests started",
		Measure:     apiRequestCount,
		TagKeys:     []tag.Key{pathKey, methodKey},
		Aggregation: view.Count(),
	}

	apiResponseCountView = &view.View{
		Name:        "api/response_count",
		Description: "API response count",
		TagKeys:     []tag.Key{pathKey, methodKey, statusKey},
		Measure:     apiLatency,
		Aggregation: view.Count(),
	}

	apiLatencyView = &view.View{
		Name:        "api/latency",
		Description: "Latency distribution of API requests",
		Measure:     apiLatency,
		TagKeys:     []tag.Key{pathKey, methodKey, statusKey},
		Aggregation: defaultLatencyDistribution,
	}
)

func registerViews() {
	err := view.Register(
		apiRequestCountView,
		apiResponseCountView,
		apiLatencyView,
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}

func makeKey(name string) tag.Key {
	key, err := tag.NewKey(name)
	if err != nil {
		logrus.Fatal(err)
	}
	return key
}
