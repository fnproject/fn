package agent

import (
	"context"
	"github.com/fnproject/fn/api/common"
)

func StatsEnqueue(ctx context.Context) {
	common.IncrementGauge(ctx, queuedMetricName)
	common.IncrementCounter(ctx, callsMetricName)
}

// Call when a function has been queued but cannot be started because of an error
func StatsDequeue(ctx context.Context) {
	common.DecrementGauge(ctx, queuedMetricName)
}

func StatsDequeueAndStart(ctx context.Context) {
	common.DecrementGauge(ctx, queuedMetricName)
	common.IncrementGauge(ctx, runningMetricName)
}

func StatsComplete(ctx context.Context) {
	common.DecrementGauge(ctx, runningMetricName)
	common.IncrementCounter(ctx, completedMetricName)
}

func StatsFailed(ctx context.Context) {
	common.DecrementGauge(ctx, runningMetricName)
	common.IncrementCounter(ctx, failedMetricName)
}

func StatsDequeueAndFail(ctx context.Context) {
	common.DecrementGauge(ctx, queuedMetricName)
	common.IncrementCounter(ctx, failedMetricName)
}

func StatsIncrementTimedout(ctx context.Context) {
	common.IncrementCounter(ctx, timedoutMetricName)
}

func StatsIncrementErrors(ctx context.Context) {
	common.IncrementCounter(ctx, errorsMetricName)
}

func StatsIncrementTooBusy(ctx context.Context) {
	common.IncrementCounter(ctx, serverBusyMetricName)
}

const (
	queuedMetricName     = "queued"
	callsMetricName      = "calls"
	runningMetricName    = "running"
	completedMetricName  = "completed"
	failedMetricName     = "failed"
	timedoutMetricName   = "timeouts"
	errorsMetricName     = "errors"
	serverBusyMetricName = "server_busy"
)
