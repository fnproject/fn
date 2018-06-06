package agent

import (
	"context"
	"strings"

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
	appKey  = makeKey("fn_appname")
	pathKey = makeKey("fn_path")
)

var (
	queuedMeasure = makeMeasure(queuedMetricName, "calls currently queued against agent", "")
	// TODO this is a dupe of sum {complete,failed} ?
	callsMeasure           = makeMeasure(callsMetricName, "calls created in agent", "")
	runningMeasure         = makeMeasure(runningMetricName, "calls currently running in agent", "")
	completedMeasure       = makeMeasure(completedMetricName, "calls completed in agent", "")
	failedMeasure          = makeMeasure(failedMetricName, "calls failed in agent", "")
	timedoutMeasure        = makeMeasure(timedoutMetricName, "calls timed out in agent", "")
	errorsMeasure          = makeMeasure(errorsMetricName, "calls errored in agent", "")
	serverBusyMeasure      = makeMeasure(serverBusyMetricName, "calls where server was too busy in agent", "")
	dockerMeasures         map[string]*stats.Int64Measure
	containerGaugeMeasures []*stats.Int64Measure
	containerTimeMeasures  []*stats.Int64Measure
)

var (
	queuedMeasureSumView     = createView(queuedMeasure, view.Sum())
	callsMeasureSumView      = createView(callsMeasure, view.Sum())
	runningMeasureSumView    = createView(runningMeasure, view.Sum())
	completedMeasureSumView  = createView(completedMeasure, view.Sum())
	failedMeasureSumView     = createView(failedMeasure, view.Sum())
	timedoutMeasureSumView   = createView(timedoutMeasure, view.Sum())
	errorsMeasureSumView     = createView(errorsMeasure, view.Sum())
	serverBusyMeasureSumView = createView(serverBusyMeasure, view.Sum())
	dockerViews              []*view.View
	containerGaugeViews      []*view.View
	containerTimeViews       []*view.View
)

func init() {
	initDockerStats()
	initContainerStats()
}

// initDockerStats initializes Docker related measures and views
func initDockerStats() {
	// TODO this is nasty figure out how to use opencensus to not have to declare these
	keys := []string{"net_rx", "net_tx", "mem_limit", "mem_usage", "disk_read", "disk_write", "cpu_user", "cpu_total", "cpu_kernel"}
	dockerMeasures := make(map[string]*stats.Int64Measure, len(keys))
	dockerViews := make([]*view.View, len(keys))
	for _, key := range keys {
		units := "bytes"
		if strings.Contains(key, "cpu") {
			units = "cpu"
		}
		dockerMeasures[key] = makeMeasure("docker_stats_"+key, "docker container stats for "+key, units)
		dockerViews = append(dockerViews, createView(dockerMeasures[key], view.Distribution()))
	}
}

// initContainerStats initializes container related measures and views
func initContainerStats() {
	// TODO(reed): do we have to do this? the measurements will be tagged on the context, will they be propagated
	// or we have to white list them in the view for them to show up? test...

	containerGaugeMeasures = make([]*stats.Int64Measure, len(containerGaugeKeys))
	containerGaugeViews = make([]*view.View, len(containerGaugeKeys))
	for i, key := range containerGaugeKeys {
		if key == "" { // leave nil intentionally, let it panic
			continue
		}
		containerGaugeMeasures[i] = makeMeasure(key, "containers in state "+key, "")
		containerGaugeViews = append(containerGaugeViews, createView(containerGaugeMeasures[i], view.Count()))
	}

	containerTimeMeasures = make([]*stats.Int64Measure, len(containerTimeKeys))
	containerTimeViews = make([]*view.View, len(containerTimeKeys))

	for i, key := range containerTimeKeys {
		if key == "" {
			continue
		}
		containerTimeMeasures[i] = makeMeasure(key, "time spent in container state "+key, "ms")
		containerTimeViews = append(containerTimeViews, createView(containerTimeMeasures[i], view.Distribution()))
	}
}

// TODO: Where to call this
func registerContainerViews() {
	for _, v := range containerGaugeViews {
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}

	for _, v := range containerTimeViews {
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}
}

func createView(measure stats.Measure, agg *view.Aggregation) *view.View {
	return &view.View{
		Name:        measure.Name(),
		Description: measure.Description(),
		Measure:     measure,
		TagKeys:     []tag.Key{appKey, pathKey},
		Aggregation: agg,
	}
}

// TODO: Where to call this
func registerViews() {
	err := view.Register(
		queuedMeasureSumView,
		callsMeasureSumView,
		runningMeasureSumView,
		completedMeasureSumView,
		failedMeasureSumView,
		timedoutMeasureSumView,
		errorsMeasureSumView,
		serverBusyMeasureSumView,
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}

// TODO: Where to call this
func registerDockerViews() {
	for _, v := range dockerViews {
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}
}

func makeMeasure(name string, desc string, unit string) *stats.Int64Measure {
	return stats.Int64(name, desc, unit)
}

func makeKey(name string) tag.Key {
	key, err := tag.NewKey(name)
	if err != nil {
		logrus.Fatal(err)
	}
	return key
}
