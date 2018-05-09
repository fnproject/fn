package agent

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// TODO add some suga:
// * hot containers active
// * memory used / available

func statsEnqueue(ctx context.Context) {
	stats.Record(ctx, queuedMeasure.M(1))
	stats.Record(ctx, callsMeasure.M(1))
}

// Call when a function has been queued but cannot be started because of an error
func statsDequeue(ctx context.Context) {
	stats.Record(ctx, queuedMeasure.M(-1))
}

func statsDequeueAndStart(ctx context.Context) {
	stats.Record(ctx, queuedMeasure.M(-1))
	stats.Record(ctx, runningMeasure.M(1))
}

func statsComplete(ctx context.Context) {
	stats.Record(ctx, runningMeasure.M(-1))
	stats.Record(ctx, completedMeasure.M(1))
}

func statsFailed(ctx context.Context) {
	stats.Record(ctx, runningMeasure.M(-1))
	stats.Record(ctx, failedMeasure.M(1))
}

func statsDequeueAndFail(ctx context.Context) {
	stats.Record(ctx, queuedMeasure.M(-1))
	stats.Record(ctx, failedMeasure.M(1))
}

func statsTimedout(ctx context.Context) {
	stats.Record(ctx, timedoutMeasure.M(1))
}

func statsErrors(ctx context.Context) {
	stats.Record(ctx, errorsMeasure.M(1))
}

func statsTooBusy(ctx context.Context) {
	stats.Record(ctx, serverBusyMeasure.M(1))
}

const (
	// TODO we should probably prefix these with calls_ ?
	queuedMetricName     = "queued"
	callsMetricName      = "calls" // TODO this is a dupe of sum {complete,failed} ?
	runningMetricName    = "running"
	completedMetricName  = "completed"
	failedMetricName     = "failed"
	timedoutMetricName   = "timeouts"
	errorsMetricName     = "errors"
	serverBusyMetricName = "server_busy"
)

var (
	queuedMeasure     *stats.Int64Measure
	callsMeasure      *stats.Int64Measure // TODO this is a dupe of sum {complete,failed} ?
	runningMeasure    *stats.Int64Measure
	completedMeasure  *stats.Int64Measure
	failedMeasure     *stats.Int64Measure
	timedoutMeasure   *stats.Int64Measure
	errorsMeasure     *stats.Int64Measure
	serverBusyMeasure *stats.Int64Measure
)

func init() {
	queuedMeasure = makeMeasure(queuedMetricName, "calls currently queued against agent", "", view.Sum())
	callsMeasure = makeMeasure(callsMetricName, "calls created in agent", "", view.Sum())
	runningMeasure = makeMeasure(runningMetricName, "calls currently running in agent", "", view.Sum())
	completedMeasure = makeMeasure(completedMetricName, "calls completed in agent", "", view.Sum())
	failedMeasure = makeMeasure(failedMetricName, "calls failed in agent", "", view.Sum())
	timedoutMeasure = makeMeasure(timedoutMetricName, "calls timed out in agent", "", view.Sum())
	errorsMeasure = makeMeasure(errorsMetricName, "calls errored in agent", "", view.Sum())
	serverBusyMeasure = makeMeasure(serverBusyMetricName, "calls where server was too busy in agent", "", view.Sum())
}

func makeMeasure(name string, desc string, unit string, agg *view.Aggregation) *stats.Int64Measure {
	appKey, err := tag.NewKey("fn_appname")
	if err != nil {
		logrus.Fatal(err)
	}
	pathKey, err := tag.NewKey("fn_path")
	if err != nil {
		logrus.Fatal(err)
	}

	measure := stats.Int64(name, desc, unit)
	err = view.Register(
		&view.View{
			Name:        name,
			Description: desc,
			TagKeys:     []tag.Key{appKey, pathKey},
			Measure:     measure,
			Aggregation: agg,
		},
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot create view")
	}
	return measure
}
