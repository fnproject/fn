package agent

import (
	"context"
	"github.com/fnproject/fn/api/common"
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

	fstats := s.getStatsForFunction(path)
	s.queue++
	fstats.queue++

	s.mu.Unlock()

	common.IncrementGauge(ctx, queuedMetricName)
	common.IncrementCounter(ctx, callsMetricName)
}

// Call when a function has been queued but cannot be started because of an error
func (s *stats) Dequeue(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForFunction(path)
	s.queue--
	fstats.queue--

	s.mu.Unlock()

	common.DecrementGauge(ctx, queuedMetricName)
}

func (s *stats) DequeueAndStart(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForFunction(path)
	s.queue--
	s.running++
	fstats.queue--
	fstats.running++

	s.mu.Unlock()

	common.DecrementGauge(ctx, queuedMetricName)
	common.IncrementGauge(ctx, runningMetricName)
}

func (s *stats) Complete(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForFunction(path)
	s.running--
	s.complete++
	fstats.running--
	fstats.complete++

	s.mu.Unlock()

	common.DecrementGauge(ctx, runningMetricName)
	common.IncrementCounter(ctx, completedMetricName)
}

func (s *stats) Failed(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForFunction(path)
	s.running--
	s.failed++
	fstats.running--
	fstats.failed++

	s.mu.Unlock()

	common.DecrementGauge(ctx, runningMetricName)
	common.IncrementCounter(ctx, failedMetricName)
}

func (s *stats) DequeueAndFail(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForFunction(path)
	s.queue--
	s.failed++
	fstats.queue--
	fstats.failed++

	s.mu.Unlock()

	common.DecrementGauge(ctx, queuedMetricName)
	common.IncrementCounter(ctx, failedMetricName)
}

func IncrementTimedout(ctx context.Context) {
	common.IncrementCounter(ctx, timedoutMetricName)
}

func IncrementErrors(ctx context.Context) {
	common.IncrementCounter(ctx, errorsMetricName)
}

func IncrementTooBusy(ctx context.Context) {
	common.IncrementCounter(ctx, serverBusyMetricName)
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
