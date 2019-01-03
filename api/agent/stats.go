package agent

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/fnproject/fn/api/common"

	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	containerStateKey    = common.MakeKey("container_state")
	callStatusKey        = common.MakeKey("call_status")
	containerUDSStateKey = common.MakeKey("container_uds_state")

	// bool flags of status call results (cached=true/false, success=true/false)
	statusCallCacheKey   = common.MakeKey("cached")
	statusCallSuccessKey = common.MakeKey("success")
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

func statsContainerUDSInitLatency(ctx context.Context, start time.Time, end time.Time, containerUDSState string) {
	if end.Before(start) {
		return
	}

	ctx, err := tag.New(ctx,
		tag.Upsert(containerUDSStateKey, containerUDSState),
	)
	if err != nil {
		logrus.Fatal(err)
	}

	dur := end.Sub(start)
	stats.Record(ctx, containerUDSInitLatencyMeasure.M(int64(dur/time.Millisecond)))
}

func statsContainerEvicted(ctx context.Context, containerState string) {
	ctx, err := tag.New(ctx,
		tag.Upsert(containerStateKey, containerState),
	)
	if err != nil {
		logrus.Fatal(err)
	}

	stats.Record(ctx, containerEvictedMeasure.M(0))
}

func statsUtilization(ctx context.Context, util ResourceUtilization) {
	stats.Record(ctx, utilCpuUsedMeasure.M(int64(util.CpuUsed)))
	stats.Record(ctx, utilCpuAvailMeasure.M(int64(util.CpuAvail)))
	stats.Record(ctx, utilMemUsedMeasure.M(int64(util.MemUsed)))
	stats.Record(ctx, utilMemAvailMeasure.M(int64(util.MemAvail)))
}

func statsCallLatency(ctx context.Context, dur time.Duration, callStatus string) {
	ctx, err := tag.New(ctx,
		tag.Upsert(callStatusKey, callStatus),
	)
	if err != nil {
		logrus.Fatal(err)
	}
	stats.Record(ctx, callLatencyMeasure.M(int64(dur/time.Millisecond)))
}

func statsStatusCall(ctx context.Context, isCached, isSuccess bool) {
	ctx, err := tag.New(ctx,
		tag.Upsert(statusCallCacheKey, strconv.FormatBool(isCached)),
		tag.Upsert(statusCallSuccessKey, strconv.FormatBool(isSuccess)),
	)
	if err != nil {
		logrus.Fatal(err)
	}
	stats.Record(ctx, statusCallMeasure.M(0))
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

	containerEvictedMetricName        = "container_evictions"
	containerUDSInitLatencyMetricName = "container_uds_init_latency"

	utilCpuUsedMetricName  = "util_cpu_used"
	utilCpuAvailMetricName = "util_cpu_avail"
	utilMemUsedMetricName  = "util_mem_used"
	utilMemAvailMetricName = "util_mem_avail"

	// Reported By LB
	runnerSchedLatencyMetricName = "lb_runner_sched_latency"
	runnerExecLatencyMetricName  = "lb_runner_exec_latency"
	callLatencyMetricName        = "lb_call_latency"

	// Reported by Runner
	statusCallMetricName = "status_call"
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

	utilCpuUsedMeasure  = common.MakeMeasure(utilCpuUsedMetricName, "agent cpu in use", "")
	utilCpuAvailMeasure = common.MakeMeasure(utilCpuAvailMetricName, "agent cpu available", "")
	utilMemUsedMeasure  = common.MakeMeasure(utilMemUsedMetricName, "agent memory in use", "By")
	utilMemAvailMeasure = common.MakeMeasure(utilMemAvailMetricName, "agent memory available", "By")

	containerEvictedMeasure        = common.MakeMeasure(containerEvictedMetricName, "containers evicted", "")
	containerUDSInitLatencyMeasure = common.MakeMeasure(containerUDSInitLatencyMetricName, "container UDS Init-Wait Latency", "msecs")

	// Reported By LB: How long does a runner scheduler wait for a committed call? eg. wait/launch/pull containers
	runnerSchedLatencyMeasure = common.MakeMeasure(runnerSchedLatencyMetricName, "Runner Scheduler Latency Reported By LBAgent", "msecs")
	// Reported By LB: Function execution time inside a container.
	runnerExecLatencyMeasure = common.MakeMeasure(runnerExecLatencyMetricName, "Runner Container Execution Latency Reported By LBAgent", "msecs")
	// Reported By LB: Function total call latency (except function execution inside container)
	callLatencyMeasure = common.MakeMeasure(callLatencyMetricName, "LB Call Latency Reported By LBAgent", "msecs")
	// Reported By Runner: Status Call Results
	statusCallMeasure = common.MakeMeasure(statusCallMetricName, "Status Call Results Reported By Runner", "")
)

func RegisterLBAgentViews(tagKeys []string, latencyDist []float64) {
	// add call_status tag for call latency
	callLatencyTags := make([]string, 0, len(tagKeys)+1)
	callLatencyTags = append(callLatencyTags, "call_status")
	for _, key := range tagKeys {
		if key != "call_status" {
			callLatencyTags = append(callLatencyTags, key)
		}
	}

	err := view.Register(
		common.CreateView(runnerSchedLatencyMeasure, view.Distribution(latencyDist...), tagKeys),
		common.CreateView(runnerExecLatencyMeasure, view.Distribution(latencyDist...), tagKeys),
		common.CreateView(callLatencyMeasure, view.Distribution(latencyDist...), callLatencyTags),
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
		common.CreateView(utilCpuUsedMeasure, view.LastValue(), tagKeys),
		common.CreateView(utilCpuAvailMeasure, view.LastValue(), tagKeys),
		common.CreateView(utilMemUsedMeasure, view.LastValue(), tagKeys),
		common.CreateView(utilMemAvailMeasure, view.LastValue(), tagKeys),
	)
	if err != nil {
		logrus.WithError(err).Fatal("cannot register view")
	}
}

// RegisterRunnerViews creates and registers all runner views
func RegisterRunnerViews(tagKeys []string, latencyDist []float64) {

	// add status call tags for status call latency
	statusCallTags := make([]string, 0, len(tagKeys)+2)

	statusCallTags = append(statusCallTags, "cached")
	statusCallTags = append(statusCallTags, "success")

	for _, key := range tagKeys {
		if key != "cached" && key != "success" {
			statusCallTags = append(statusCallTags, key)
		}
	}

	err := view.Register(
		common.CreateView(statusCallMeasure, view.Count(), statusCallTags),
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

	// add container state tag for evictions
	evictTags := make([]string, 0, len(tagKeys)+1)
	evictTags = append(evictTags, "container_state")
	for _, key := range tagKeys {
		if key != "container_state" {
			evictTags = append(evictTags, key)
		}
	}

	// add container uds_state tag for uds-wait
	udsInitTags := make([]string, 0, len(tagKeys)+1)
	udsInitTags = append(udsInitTags, "container_uds_state")
	for _, key := range tagKeys {
		if key != "container_uds_state" {
			udsInitTags = append(udsInitTags, key)
		}
	}

	err := view.Register(
		common.CreateView(containerEvictedMeasure, view.Count(), evictTags),
		common.CreateView(containerUDSInitLatencyMeasure, view.Distribution(latencyDist...), udsInitTags),
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
