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
	queuedMeasure = makeMeasure(queuedMetricName, "calls currently queued against agent", "")
	// TODO this is a dupe of sum {complete,failed} ?
	callsMeasure           = makeMeasure(callsMetricName, "calls created in agent", "")
	runningMeasure         = makeMeasure(runningMetricName, "calls currently running in agent", "")
	completedMeasure       = makeMeasure(completedMetricName, "calls completed in agent", "")
	failedMeasure          = makeMeasure(failedMetricName, "calls failed in agent", "")
	timedoutMeasure        = makeMeasure(timedoutMetricName, "calls timed out in agent", "")
	errorsMeasure          = makeMeasure(errorsMetricName, "calls errored in agent", "")
	serverBusyMeasure      = makeMeasure(serverBusyMetricName, "calls where server was too busy in agent", "")
	dockerMeasures         = initDockerMeasures()
	containerGaugeMeasures = initContainerGaugeMeasures()
	containerTimeMeasures  = initContainerTimeMeasures()
)

// RegisterAgentViews creates and registers all agent views
func RegisterAgentViews(tagKeys []string) {
	err := view.Register(
		createView(queuedMeasure, view.Sum(), tagKeys),
		createView(callsMeasure, view.Sum(), tagKeys),
		createView(runningMeasure, view.Sum(), tagKeys),
		createView(completedMeasure, view.Sum(), tagKeys),
		createView(failedMeasure, view.Sum(), tagKeys),
		createView(timedoutMeasure, view.Sum(), tagKeys),
		createView(errorsMeasure, view.Sum(), tagKeys),
		createView(serverBusyMeasure, view.Sum(), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}

// RegisterDockerViews creates a and registers Docker views with provided tag keys
func RegisterDockerViews(tagKeys []string) {
	for _, m := range dockerMeasures {
		v := createView(m, view.Distribution(), tagKeys)
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}
}

// RegisterContainerViews creates and register containers views with provided tag keys
func RegisterContainerViews(tagKeys []string) {
	// Create views for container measures
	for i, key := range containerGaugeKeys {
		if key == "" {
			continue
		}
		v := createView(containerGaugeMeasures[i], view.Count(), tagKeys)
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}

	for i, key := range containerTimeKeys {
		if key == "" {
			continue
		}
		v := createView(containerTimeMeasures[i], view.Distribution(), tagKeys)
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}
}

// initDockerMeasures initializes Docker related measures
func initDockerMeasures() map[string]*stats.Int64Measure {
	// TODO this is nasty figure out how to use opencensus to not have to declare these
	keys := []string{"net_rx", "net_tx", "mem_limit", "mem_usage", "disk_read", "disk_write", "cpu_user", "cpu_total", "cpu_kernel"}
	measures := make(map[string]*stats.Int64Measure, len(keys))
	for _, key := range keys {
		units := "bytes"
		if strings.Contains(key, "cpu") {
			units = "cpu"
		}
		measures[key] = makeMeasure("docker_stats_"+key, "docker container stats for "+key, units)
	}
	return measures
}

func initContainerGaugeMeasures() []*stats.Int64Measure {
	gaugeMeasures := make([]*stats.Int64Measure, len(containerGaugeKeys))
	for i, key := range containerGaugeKeys {
		if key == "" { // leave nil intentionally, let it panic
			continue
		}
		gaugeMeasures[i] = makeMeasure(key, "containers in state "+key, "")
	}
	return gaugeMeasures
}

func initContainerTimeMeasures() []*stats.Int64Measure {
	// TODO(reed): do we have to do this? the measurements will be tagged on the context, will they be propagated
	// or we have to white list them in the view for them to show up? test...

	timeMeasures := make([]*stats.Int64Measure, len(containerTimeKeys))
	for i, key := range containerTimeKeys {
		if key == "" {
			continue
		}
		timeMeasures[i] = makeMeasure(key, "time spent in container state "+key, "ms")
	}

	return timeMeasures
}

func createView(measure stats.Measure, agg *view.Aggregation, tagKeys []string) *view.View {
	return &view.View{
		Name:        measure.Name(),
		Description: measure.Description(),
		Measure:     measure,
		TagKeys:     makeKeys(tagKeys),
		Aggregation: agg,
	}
}

func makeMeasure(name string, desc string, unit string) *stats.Int64Measure {
	return stats.Int64(name, desc, unit)
}

func makeKeys(names []string) []tag.Key {
	tagKeys := make([]tag.Key, len(names))
	for i, name := range names {
		key, err := tag.NewKey(name)
		if err != nil {
			logrus.Fatal(err)
		}
		tagKeys[i] = key
	}
	return tagKeys
}
