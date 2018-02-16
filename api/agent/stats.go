package agent

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
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
	// TODO(reed): doing this at each call site seems not the intention of the library since measurements
	// need to be created and views registered. doing this up front seems painful but maybe there
	// are benefits?

	// TODO(reed): do we have to do this? the measurements will be tagged on the context, will they be propagated
	// or we have to white list them in the view for them to show up? test...
	var err error
	//appKey, err := tag.NewKey("fn_appname")
	//if err != nil {
	//logrus.Fatal(err)
	//}
	//pathKey, err := tag.NewKey("fn_path")
	//if err != nil {
	//logrus.Fatal(err)
	//}

	{
		queuedMeasure, err = stats.Int64(queuedMetricName, "calls currently queued against agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			queuedMetricName,
			"calls currently queued to agent",
			nil, // []tag.Key{appKey, pathKey},
			queuedMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		callsMeasure, err = stats.Int64(callsMetricName, "calls created in agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			callsMetricName,
			"calls created in agent",
			nil, // []tag.Key{appKey, pathKey},
			callsMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		runningMeasure, err = stats.Int64(runningMetricName, "calls currently running in agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			runningMetricName,
			"calls currently running in agent",
			nil, // []tag.Key{appKey, pathKey},
			runningMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		completedMeasure, err = stats.Int64(completedMetricName, "calls completed in agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			completedMetricName,
			"calls completed in agent",
			nil, // []tag.Key{appKey, pathKey},
			completedMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		failedMeasure, err = stats.Int64(failedMetricName, "calls failed in agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			failedMetricName,
			"calls failed in agent",
			nil, // []tag.Key{appKey, pathKey},
			failedMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		timedoutMeasure, err = stats.Int64(timedoutMetricName, "calls timed out in agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			timedoutMetricName,
			"calls timed out in agent",
			nil, // []tag.Key{appKey, pathKey},
			timedoutMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		errorsMeasure, err = stats.Int64(errorsMetricName, "calls errored in agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			errorsMetricName,
			"calls errored in agent",
			nil, // []tag.Key{appKey, pathKey},
			errorsMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}

	{
		serverBusyMeasure, err = stats.Int64(serverBusyMetricName, "calls where server was too busy in agent", "")
		if err != nil {
			logrus.Fatal(err)
		}
		v, err := view.New(
			serverBusyMetricName,
			"calls where server was too busy in agent",
			nil, // []tag.Key{appKey, pathKey},
			serverBusyMeasure,
			view.SumAggregation{},
		)
		if err != nil {
			logrus.Fatalf("cannot create view: %v", err)
		}
		if err := v.Subscribe(); err != nil {
			logrus.Fatal(err)
		}
	}
}
