package agent

import (
	"context"

	"github.com/fnproject/fn/api/common"
)

// TODO add some suga:
// * hot containers active
// * memory used / available

func statsEnqueue(ctx context.Context) {
	common.IncrementGauge(ctx, queuedMetricName)
	common.IncrementCounter(ctx, callsMetricName)
}

// Call when a function has been queued but cannot be started because of an error
func statsDequeue(ctx context.Context) {
	common.DecrementGauge(ctx, queuedMetricName)
}

func statsDequeueAndStart(ctx context.Context) {
	common.DecrementGauge(ctx, queuedMetricName)
	common.IncrementGauge(ctx, runningMetricName)
}

func statsComplete(ctx context.Context) {
	common.DecrementGauge(ctx, runningMetricName)
	common.IncrementCounter(ctx, completedMetricName)
}

func statsFailed(ctx context.Context) {
	common.DecrementGauge(ctx, runningMetricName)
	common.IncrementCounter(ctx, failedMetricName)
}

func statsDequeueAndFail(ctx context.Context) {
	common.DecrementGauge(ctx, queuedMetricName)
	common.IncrementCounter(ctx, failedMetricName)
}

func statsTimedout(ctx context.Context) {
	common.IncrementCounter(ctx, timedoutMetricName)
}

func statsErrors(ctx context.Context) {
	common.IncrementCounter(ctx, errorsMetricName)
}

func statsTooBusy(ctx context.Context) {
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
