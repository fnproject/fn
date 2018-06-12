package runnerpool

import (
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	fullScanCountMeasure     = stats.Int64("lb_placer_fullscan_count", "LB Placer Full RunnerList Scan Count", "")
	errorPoolCountMeasure    = stats.Int64("lb_placer_rp_error_count", "LB Placer RunnerPool RunnerList Error Count", "")
	emptyPoolCountMeasure    = stats.Int64("lb_placer_rp_empty_count", "LB Placer RunnerPool RunnerList Empty Count", "")
	cancelCountMeasure       = stats.Int64("lb_placer_client_cancelled_count", "LB Placer Client Cancel Count", "")
	placedErrorCountMeasure  = stats.Int64("lb_placer_placed_error_count", "LB Placer Placed Call Count With Errors", "")
	placedOKCountMeasure     = stats.Int64("lb_placer_placed_ok_count", "LB Placer Placed Call Count Without Errors", "")
	retryTooBusyCountMeasure = stats.Int64("lb_placer_retry_busy_count", "LB Placer Retry Count - Too Busy", "")
	retryErrorCountMeasure   = stats.Int64("lb_placer_retry_error_count", "LB Placer Retry Count - Errors", "")
	placerLatencyMeasure     = stats.Int64("lb_placer_latency", "LB Placer Latency", "msecs")
)

func makeKeys(names []string) []tag.Key {
	var tagKeys []tag.Key
	for _, name := range names {
		key, err := tag.NewKey(name)
		if err != nil {
			logrus.WithError(err).Fatal("cannot create tag key for %v", name)
		}
		tagKeys = append(tagKeys, key)
	}
	return tagKeys
}

func createView(measure stats.Measure, agg *view.Aggregation, tagKeys []string) *view.View {
	return &view.View{
		Name:        measure.Name(),
		Description: measure.Description(),
		TagKeys:     makeKeys(tagKeys),
		Measure:     measure,
		Aggregation: agg,
	}
}

func RegisterPlacerViews(tagKeys []string) {
	err := view.Register(
		createView(fullScanCountMeasure, view.Count(), tagKeys),
		createView(errorPoolCountMeasure, view.Count(), tagKeys),
		createView(emptyPoolCountMeasure, view.Count(), tagKeys),
		createView(cancelCountMeasure, view.Count(), tagKeys),
		createView(placedErrorCountMeasure, view.Count(), tagKeys),
		createView(placedOKCountMeasure, view.Count(), tagKeys),
		createView(retryTooBusyCountMeasure, view.Count(), tagKeys),
		createView(retryErrorCountMeasure, view.Count(), tagKeys),
		createView(placerLatencyMeasure, view.Distribution(1, 10, 25, 50, 200, 1000, 10000, 60000), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot create view")
	}
}
