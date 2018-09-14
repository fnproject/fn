package agent

import (
	"context"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

func statsCalls(ctx context.Context) {
	stats.Record(ctx, callsMeasure.M(1))
}

func statsEnqueue(ctx context.Context) {
	stats.Record(ctx, queuedMeasure.M(1))
}

func statsDequeue(ctx context.Context) {
	stats.Record(ctx, queuedMeasure.M(-1))
}

func statsStartRun(ctx context.Context) {
	stats.Record(ctx, runningMeasure.M(1))
}

func statsStopRun(ctx context.Context) {
	stats.Record(ctx, runningMeasure.M(-1))
}

func statsComplete(ctx context.Context) {
	stats.Record(ctx, completedMeasure.M(1))
}

func statsCanceled(ctx context.Context) {
	stats.Record(ctx, canceledMeasure.M(1))
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

func statsLBAgentRunnerSchedLatency(ctx context.Context, dur time.Duration) {
	stats.Record(ctx, runnerSchedLatencyMeasure.M(int64(dur/time.Millisecond)))
}

func statsLBAgentRunnerExecLatency(ctx context.Context, dur time.Duration) {
	stats.Record(ctx, runnerExecLatencyMeasure.M(int64(dur/time.Millisecond)))
}

func statsContainerEvicted(ctx context.Context) {
	stats.Record(ctx, containerEvictedMeasure.M(0))
}

const (
	//
	// WARNING: Dual Role Metrics both used in Runner/Agent and LB-Agent
	//
	// LB Context:
	//
	// calls - call received in Agent Submit
	// queued - LB is reading request from Client and attempting to validate/start
	// running - LB is forwarding Call to runners
	// completed - call completed running successfully
	// canceled - call canceled (client disconnect)
	// timeouts - call timed out
	// errors - call failed
	// server_busy - server busy responses (retriable)
	//
	// Agent/Runner Context:
	//
	// calls - calls received in Agent Submit
	// queued - Reading/validating call from client and waiting for resources/containers to start
	// running - call is now running
	// completed - call completed running (success)
	// canceled - call canceled (client disconnect)
	// timeouts - call timed out
	// errors - call failed
	// server_busy - server busy responses (retriable)
	//
	queuedMetricName     = "queued"
	callsMetricName      = "calls"
	runningMetricName    = "running"
	completedMetricName  = "completed"
	canceledMetricName   = "canceled"
	timedoutMetricName   = "timeouts"
	errorsMetricName     = "errors"
	serverBusyMetricName = "server_busy"

	containerEvictedMetricName = "container_evictions"

	// Reported By LB
	runnerSchedLatencyMetricName = "lb_runner_sched_latency"
	runnerExecLatencyMetricName  = "lb_runner_exec_latency"
)

var (
	queuedMeasure          = common.MakeMeasure(queuedMetricName, "calls currently queued against agent", "")
	callsMeasure           = common.MakeMeasure(callsMetricName, "calls created in agent", "")
	runningMeasure         = common.MakeMeasure(runningMetricName, "calls currently running in agent", "")
	completedMeasure       = common.MakeMeasure(completedMetricName, "calls completed in agent", "")
	canceledMeasure        = common.MakeMeasure(canceledMetricName, "calls canceled in agent", "")
	timedoutMeasure        = common.MakeMeasure(timedoutMetricName, "calls timed out in agent", "")
	errorsMeasure          = common.MakeMeasure(errorsMetricName, "calls errored in agent", "")
	serverBusyMeasure      = common.MakeMeasure(serverBusyMetricName, "calls where server was too busy in agent", "")
	dockerMeasures         = initDockerMeasures()
	containerGaugeMeasures = initContainerGaugeMeasures()
	containerTimeMeasures  = initContainerTimeMeasures()

	containerEvictedMeasure = common.MakeMeasure(containerEvictedMetricName, "containers evicted", "")

	// Reported By LB: How long does a runner scheduler wait for a committed call? eg. wait/launch/pull containers
	runnerSchedLatencyMeasure = common.MakeMeasure(runnerSchedLatencyMetricName, "Runner Scheduler Latency Reported By LBAgent", "msecs")
	// Reported By LB: Function execution time inside a container.
	runnerExecLatencyMeasure = common.MakeMeasure(runnerExecLatencyMetricName, "Runner Container Execution Latency Reported By LBAgent", "msecs")
)

func RegisterLBAgentViews(tagKeys []string, latencyDist []float64) {
	err := view.Register(
		common.CreateView(runnerSchedLatencyMeasure, view.Distribution(latencyDist...), tagKeys),
		common.CreateView(runnerExecLatencyMeasure, view.Distribution(latencyDist...), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}

// RegisterAgentViews creates and registers all agent views
func RegisterAgentViews(tagKeys []string, latencyDist []float64) {
	err := view.Register(
		common.CreateView(queuedMeasure, view.Sum(), tagKeys),
		common.CreateView(callsMeasure, view.Sum(), tagKeys),
		common.CreateView(runningMeasure, view.Sum(), tagKeys),
		common.CreateView(completedMeasure, view.Sum(), tagKeys),
		common.CreateView(canceledMeasure, view.Sum(), tagKeys),
		common.CreateView(timedoutMeasure, view.Sum(), tagKeys),
		common.CreateView(errorsMeasure, view.Sum(), tagKeys),
		common.CreateView(serverBusyMeasure, view.Sum(), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}

// RegisterDockerViews creates a and registers Docker views with provided tag keys
func RegisterDockerViews(tagKeys []string, latencyDist, ioNetDist, ioDiskDist, memoryDist, cpuDist []float64) {

	for _, m := range dockerMeasures {

		var dist *view.Aggregation

		// Remember these are sampled by docker in short intervals (approx 1 sec)
		if m.Name() == "docker_stats_net_rx" || m.Name() == "docker_stats_net_tx" {
			dist = view.Distribution(ioNetDist...)
		} else if m.Name() == "docker_stats_disk_read" || m.Name() == "docker_stats_disk_write" {
			dist = view.Distribution(ioDiskDist...)
		} else if m.Name() == "docker_stats_mem_limit" || m.Name() == "docker_stats_mem_usage" {
			dist = view.Distribution(memoryDist...)
		} else if m.Name() == "docker_stats_cpu_user" || m.Name() == "docker_stats_cpu_total" || m.Name() == "docker_stats_cpu_kernel" {
			dist = view.Distribution(cpuDist...)
		} else {
			// Not used yet.
			dist = view.Distribution(latencyDist...)
		}

		v := common.CreateView(m, dist, tagKeys)
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}
}

// RegisterContainerViews creates and register containers views with provided tag keys
func RegisterContainerViews(tagKeys []string, latencyDist []float64) {
	// Create views for container measures
	for i, key := range containerGaugeKeys {
		if key == "" {
			continue
		}
		v := common.CreateView(containerGaugeMeasures[i], view.Sum(), tagKeys)
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}

	for i, key := range containerTimeKeys {
		if key == "" {
			continue
		}
		v := common.CreateView(containerTimeMeasures[i], view.Distribution(latencyDist...), tagKeys)
		if err := view.Register(v); err != nil {
			logrus.WithError(err).Fatal("cannot register view")
		}
	}

	err := view.Register(
		common.CreateView(containerEvictedMeasure, view.Count(), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
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
		measures[key] = common.MakeMeasure("docker_stats_"+key, "docker container stats for "+key, units)
	}
	return measures
}

func initContainerGaugeMeasures() []*stats.Int64Measure {
	gaugeMeasures := make([]*stats.Int64Measure, len(containerGaugeKeys))
	for i, key := range containerGaugeKeys {
		if key == "" { // leave nil intentionally, let it panic
			continue
		}
		gaugeMeasures[i] = common.MakeMeasure(key, "containers in state "+key, "")
	}
	return gaugeMeasures
}

func initContainerTimeMeasures() []*stats.Int64Measure {
	timeMeasures := make([]*stats.Int64Measure, len(containerTimeKeys))
	for i, key := range containerTimeKeys {
		if key == "" {
			continue
		}
		timeMeasures[i] = common.MakeMeasure(key, "time spent in container state "+key, "ms")
	}

	return timeMeasures
}
