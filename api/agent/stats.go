package agent

import (
	"context"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
	"sync"
)

// TODO this should expose:
// * hot containers active
// * memory used / available

// global statistics
type stats struct {
	mu sync.Mutex
	// statistics for all functions combined
	queue    uint64
	running  uint64
	complete uint64
	failed   uint64
	// statistics for individual functions, keyed by function path
	functionStatsMap map[string]*functionStats
}

// statistics for an individual function
type functionStats struct {
	queue    uint64
	running  uint64
	complete uint64
	failed   uint64
}

// Stats hold the statistics for all functions combined
// and the statistics for each individual function
type Stats struct {
	Queue    uint64
	Running  uint64
	Complete uint64
	Failed   uint64
	// statistics for individual functions, keyed by function path
	FunctionStatsMap map[string]*FunctionStats
}

// FunctionStats holds the statistics for an individual function
type FunctionStats struct {
	Queue    uint64
	Running  uint64
	Complete uint64
	Failed   uint64
}

func (s *stats) getStatsForFunction(path string) *functionStats {
	if s.functionStatsMap == nil {
		s.functionStatsMap = make(map[string]*functionStats)
	}
	thisFunctionStats, found := s.functionStatsMap[path]
	if !found {
		thisFunctionStats = &functionStats{}
		s.functionStatsMap[path] = thisFunctionStats
	}

	return thisFunctionStats
}

func (s *stats) Enqueue(ctx context.Context, app string, path string) {
	s.mu.Lock()

	s.queue++
	s.getStatsForFunction(path).queue++
	IncrementGauge(ctx, queuedSuffix)

	IncrementCounter(ctx, callsSuffix)

	s.mu.Unlock()
}

// Call when a function has been queued but cannot be started because of an error
func (s *stats) Dequeue(ctx context.Context, app string, path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	DecrementGauge(ctx, queuedSuffix)

	s.mu.Unlock()
}

func (s *stats) DequeueAndStart(ctx context.Context, app string, path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	DecrementGauge(ctx, queuedSuffix)

	s.running++
	s.getStatsForFunction(path).running++
	IncrementGauge(ctx, runningSuffix)

	s.mu.Unlock()
}

func (s *stats) Complete(ctx context.Context, app string, path string) {
	s.mu.Lock()

	s.running--
	s.getStatsForFunction(path).running--
	DecrementGauge(ctx, runningSuffix)

	s.complete++
	s.getStatsForFunction(path).complete++
	IncrementCounter(ctx, completedSuffix)

	s.mu.Unlock()
}

func (s *stats) Failed(ctx context.Context, app string, path string) {
	s.mu.Lock()

	s.running--
	s.getStatsForFunction(path).running--
	DecrementGauge(ctx, runningSuffix)

	s.failed++
	s.getStatsForFunction(path).failed++
	IncrementCounter(ctx, failedSuffix)

	s.mu.Unlock()
}

func (s *stats) DequeueAndFail(ctx context.Context, app string, path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	DecrementGauge(ctx, queuedSuffix)

	s.failed++
	s.getStatsForFunction(path).failed++
	IncrementCounter(ctx, failedSuffix)

	s.mu.Unlock()
}

func (s *stats) Stats() Stats {
	var stats Stats
	s.mu.Lock()
	stats.Running = s.running
	stats.Complete = s.complete
	stats.Queue = s.queue
	stats.Failed = s.failed
	stats.FunctionStatsMap = make(map[string]*FunctionStats)
	for key, value := range s.functionStatsMap {
		thisFunctionStats := &FunctionStats{Queue: value.queue, Running: value.running, Complete: value.complete, Failed: value.failed}
		stats.FunctionStatsMap[key] = thisFunctionStats
	}
	s.mu.Unlock()
	return stats
}

// Constants used when constructing span names and field keys for these metrics
var queuedSuffix = "queued"
var callsSuffix = "calls"
var runningSuffix = "running"
var completedSuffix = "completed"
var failedSuffix = "failed"

// IncrementGauge increments the specified gauge metric
// It does this by logging an appropriate field value to a tracing span.
func IncrementGauge(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with "fn_gauge_" to designate that it is a Prometheus gauge metric
	// The collector will remove "gauge_" and use the result as the Prometheus metric name.
	fieldname := "fn_gauge_" + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the span is long-lived we create a new span for every call
	// suffix the span name with "_dummy" to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+"_dummy")
	defer span.Finish()

	// gauge metrics are actually float64; here we log that it should be increased by +1
	span.LogFields(log.Float64(fieldname, 1.))
}

// DecrementGauge decrements the specified gauge metric
// It does this by logging an appropriate field value to a tracing span.
func DecrementGauge(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with "fn_gauge_" to designate that it is a Prometheus gauge metric
	// The collector will remove "gauge_" and use the result as the Prometheus metric name.
	fieldname := "fn_gauge_" + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the span is long-lived we create a new span for every call
	// suffix the span name with "_dummy" to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+"_dummy")
	defer span.Finish()

	// gauge metrics are actually float64; here we log that it should be increased by -1
	span.LogFields(log.Float64(fieldname, -1.))
}

// IncrementCounter increments the specified counter metric
// It does this by logging an appropriate field value to a tracing span.
func IncrementCounter(ctx context.Context, metric string) {
	// The field name we use is the specified metric name prepended with "fn_counter_" to designate that it is a Prometheus counter metric
	// The collector will remove "fn_counter_" and use the result as the Prometheus metric name.
	fieldname := "fn_counter_" + metric

	// Spans are not processed by the collector until the span ends, so to prevent any delay
	// in processing the stats when the span is long-lived we create a new span for every call
	// suffix the span name with "_dummy" to denote that it is used only to hold a metric and isn't itself of any interest
	span, ctx := opentracing.StartSpanFromContext(ctx, fieldname+"_dummy")
	defer span.Finish()

	// counter metrics are actually float64; here we log that it should be increased by +1
	span.LogFields(log.Float64(fieldname, 1.))
}
