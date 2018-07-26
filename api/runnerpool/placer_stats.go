package runnerpool

import (
	"context"
	"math"
	"time"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	attemptCountMeasure      = stats.Int64("lb_placer_attempt_count", "LB Placer Number of Runners Attempted Count", "")
	errorPoolCountMeasure    = stats.Int64("lb_placer_rp_error_count", "LB Placer RunnerPool RunnerList Error Count", "")
	emptyPoolCountMeasure    = stats.Int64("lb_placer_rp_empty_count", "LB Placer RunnerPool RunnerList Empty Count", "")
	cancelCountMeasure       = stats.Int64("lb_placer_client_cancelled_count", "LB Placer Client Cancel Count", "")
	placerTimeoutMeasure     = stats.Int64("lb_placer_timeout_count", "LB Placer Timeout Count", "")
	placedErrorCountMeasure  = stats.Int64("lb_placer_placed_error_count", "LB Placer Placed Call Count With Errors", "")
	placedOKCountMeasure     = stats.Int64("lb_placer_placed_ok_count", "LB Placer Placed Call Count Without Errors", "")
	retryTooBusyCountMeasure = stats.Int64("lb_placer_retry_busy_count", "LB Placer Retry Count - Too Busy", "")
	retryErrorCountMeasure   = stats.Int64("lb_placer_retry_error_count", "LB Placer Retry Count - Errors", "")
	placerLatencyMeasure     = stats.Int64("lb_placer_latency", "LB Placer Latency", "msecs")
)

// Helper struct for tracking LB Placer latency and attempt counts
type attemptTracker struct {
	ctx             context.Context
	startTime       time.Time
	lastAttemptTime time.Time
	attemptCount    int64
}

func newAttemptTracker(ctx context.Context) *attemptTracker {
	return &attemptTracker{
		ctx:       ctx,
		startTime: time.Now(),
	}
}

func (data *attemptTracker) finalizeAttempts(isCommited bool) {
	stats.Record(data.ctx, attemptCountMeasure.M(data.attemptCount))

	// IMPORTANT: here we use (lastAttemptTime - startTime). We want to exclude TryExec
	// latency *if* TryExec() goes through with commit. Placer latency metric only shows
	// how much time are spending in Placer loop/retries. The metric includes rtt/latency of
	// *all* unsuccessful NACK (retriable) responses from runners as well. For example, if
	// Placer loop here retries 4 runners (which takes 5 msecs each) and then 5th runner
	// succeeds (but takes 35 seconds to finish execution), we report 20 msecs as our LB
	// latency.
	endTime := data.lastAttemptTime
	if !isCommited {
		endTime = time.Now()
	}

	stats.Record(data.ctx, placerLatencyMeasure.M(int64(endTime.Sub(data.startTime)/time.Millisecond)))
}

func (data *attemptTracker) recordAttempt() {
	data.lastAttemptTime = time.Now()
	if data.attemptCount != math.MaxInt64 {
		data.attemptCount++
	}
}

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
		createView(attemptCountMeasure, view.Distribution(0, 1, 2, 4, 8, 32, 64, 256), tagKeys),
		createView(errorPoolCountMeasure, view.Count(), tagKeys),
		createView(emptyPoolCountMeasure, view.Count(), tagKeys),
		createView(cancelCountMeasure, view.Count(), tagKeys),
		createView(placerTimeoutMeasure, view.Count(), tagKeys),
		createView(placedErrorCountMeasure, view.Count(), tagKeys),
		createView(placedOKCountMeasure, view.Count(), tagKeys),
		createView(retryTooBusyCountMeasure, view.Count(), tagKeys),
		createView(retryErrorCountMeasure, view.Count(), tagKeys),
		createView(placerLatencyMeasure, view.Distribution(1, 10, 25, 50, 200, 1000, 1500, 2000, 2500, 3000, 10000, 60000), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot create view")
	}
}
