package agent

import (
	"github.com/prometheus/client_golang/prometheus"
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

type Stats struct {
	// statistics for all functions combined
	Queue    uint64
	Running  uint64
	Complete uint64
	Failed   uint64
	// statistics for individual functions, keyed by function path
	FunctionStatsMap map[string]*FunctionStats
}

// statistics for an individual function
type FunctionStats struct {
	Queue    uint64
	Running  uint64
	Complete uint64
	Failed   uint64
}

var (
	fnQueued = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fn_api_queued",
			Help: "Queued requests by path",
		},
		[](string){"path"},
	)
	fnRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fn_api_running",
			Help: "Running requests by path",
		},
		[](string){"path"},
	)
	fnCompleted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fn_api_completed",
			Help: "Completed requests by path",
		},
		[](string){"path"},
	)
	fnFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fn_api_failed",
			Help: "Failed requests by path",
		},
		[](string){"path"},
	)
)

func init() {
	prometheus.MustRegister(fnQueued)
	prometheus.MustRegister(fnRunning)
	prometheus.MustRegister(fnFailed)
	prometheus.MustRegister(fnCompleted)
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

func (s *stats) Enqueue(path string) {
	s.mu.Lock()

	s.queue++
	s.getStatsForFunction(path).queue++
	fnQueued.WithLabelValues(path).Inc()

	s.mu.Unlock()
}

// Call when a function has been queued but cannot be started because of an error
func (s *stats) Dequeue(path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	fnQueued.WithLabelValues(path).Dec()

	s.mu.Unlock()
}

func (s *stats) DequeueAndStart(path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	fnQueued.WithLabelValues(path).Dec()

	s.running++
	s.getStatsForFunction(path).running++
	fnRunning.WithLabelValues(path).Inc()

	s.mu.Unlock()
}

func (s *stats) Complete(path string) {
	s.mu.Lock()

	s.running--
	s.getStatsForFunction(path).running--
	fnRunning.WithLabelValues(path).Dec()

	s.complete++
	s.getStatsForFunction(path).complete++
	fnCompleted.WithLabelValues(path).Inc()

	s.mu.Unlock()
}

func (s *stats) Failed(path string) {
	s.mu.Lock()

	s.running--
	s.getStatsForFunction(path).running--
	fnRunning.WithLabelValues(path).Dec()

	s.failed++
	s.getStatsForFunction(path).failed++
	fnFailed.WithLabelValues(path).Inc()

	s.mu.Unlock()
}

func (s *stats) DequeueAndFail(path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	fnQueued.WithLabelValues(path).Dec()

	s.failed++
	s.getStatsForFunction(path).failed++
	fnFailed.WithLabelValues(path).Inc()

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
