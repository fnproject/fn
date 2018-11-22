package runnerpool

import (
	"context"
	"math"
	"time"

	"github.com/fnproject/fn/api/common"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	attemptCountMeasure      = common.MakeMeasure("lb_placer_attempt_count", "LB Placer Number of Runners Attempted Count", "")
	errorPoolCountMeasure    = common.MakeMeasure("lb_placer_rp_error_count", "LB Placer RunnerPool RunnerList Error Count", "")
	emptyPoolCountMeasure    = common.MakeMeasure("lb_placer_rp_empty_count", "LB Placer RunnerPool RunnerList Empty Count", "")
	cancelCountMeasure       = common.MakeMeasure("lb_placer_client_cancelled_count", "LB Placer Client Cancel Count", "")
	timeoutCountMeasure      = common.MakeMeasure("lb_placer_client_timeout_count", "LB Placer Client Timeout Count", "")
	placerTimeoutMeasure     = common.MakeMeasure("lb_placer_timeout_count", "LB Placer Timeout Count", "")
	placedErrorCountMeasure  = common.MakeMeasure("lb_placer_placed_error_count", "LB Placer Placed Call Count With Errors", "")
	placedAbortCountMeasure  = common.MakeMeasure("lb_placer_placed_abort_count", "LB Placer Placed Call Count With Client Timeout/Cancel", "")
	placedOKCountMeasure     = common.MakeMeasure("lb_placer_placed_ok_count", "LB Placer Placed Call Count Without Errors", "")
	retryTooBusyCountMeasure = common.MakeMeasure("lb_placer_retry_busy_count", "LB Placer Retry Count - Too Busy", "")
	retryErrorCountMeasure   = common.MakeMeasure("lb_placer_retry_error_count", "LB Placer Retry Count - Errors", "")
	placerLatencyMeasure     = common.MakeMeasure("lb_placer_latency", "LB Placer Latency", "msecs")
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

func RegisterPlacerViews(tagKeys []string, latencyDist []float64) {
	err := view.Register(
		common.CreateView(attemptCountMeasure, view.Distribution(0, 2, 3, 4, 8, 16, 32, 64, 128, 256), tagKeys),
		common.CreateView(errorPoolCountMeasure, view.Count(), tagKeys),
		common.CreateView(emptyPoolCountMeasure, view.Count(), tagKeys),
		common.CreateView(timeoutCountMeasure, view.Count(), tagKeys),
		common.CreateView(cancelCountMeasure, view.Count(), tagKeys),
		common.CreateView(placerTimeoutMeasure, view.Count(), tagKeys),
		common.CreateView(placedErrorCountMeasure, view.Count(), tagKeys),
		common.CreateView(placedAbortCountMeasure, view.Count(), tagKeys),
		common.CreateView(placedOKCountMeasure, view.Count(), tagKeys),
		common.CreateView(retryTooBusyCountMeasure, view.Count(), tagKeys),
		common.CreateView(retryErrorCountMeasure, view.Count(), tagKeys),
		common.CreateView(placerLatencyMeasure, view.Distribution(latencyDist...), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot create view")
	}
}
