package server

import (
	"github.com/fnproject/fn/api/common"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	apiRequestCountMeasure  = common.MakeMeasure("api/request_count", "Count of API requests started", stats.UnitDimensionless)
	apiResponseCountMeasure = common.MakeMeasure("api/response_count", "API response count", stats.UnitDimensionless)
	apiLatencyMeasure       = common.MakeMeasure("api/latency", "Latency distribution of API requests", stats.UnitMilliseconds)
)

func RegisterAPIViews(tagKeys []string, dist []float64) {
	err := view.Register(
		common.CreateView(apiRequestCountMeasure, view.Count(), tagKeys),
		common.CreateView(apiResponseCountMeasure, view.Count(), tagKeys),
		common.CreateView(apiLatencyMeasure, view.Distribution(dist...), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}
