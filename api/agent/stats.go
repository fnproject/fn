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
	queue      uint64
	running    uint64
	complete   uint64
	failed     uint64
	containers map[string]float64
	images     map[string]float64
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
	Queue      uint64
	Running    uint64
	Complete   uint64
	Failed     uint64
	Containers map[string]float64
	Images     map[string]float64
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

type ImagePair struct {
	state string
	value float64
}

type ContainerPair struct {
	state string
	value float64
}

var (
	fnQueued = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fn_api_queued",
			Help: "Queued requests by path",
		},
		[](string){"app", "path"},
	)
	fnRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fn_api_running",
			Help: "Running requests by path",
		},
		[](string){"app", "path"},
	)
	fnCompleted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fn_api_completed",
			Help: "Completed requests by path",
		},
		[](string){"app", "path"},
	)
	fnFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fn_api_failed",
			Help: "Failed requests by path",
		},
		[](string){"app", "path"},
	)
	fnContainers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fn_containers",
			Help: "Number of containers",
		},
		[](string){"state"},
	)
	fnImages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "fn_images",
			Help: "Number of images",
		},
		[](string){"state"},
	)
)

func init() {
	prometheus.MustRegister(fnQueued)
	prometheus.MustRegister(fnRunning)
	prometheus.MustRegister(fnFailed)
	prometheus.MustRegister(fnCompleted)
	prometheus.MustRegister(fnContainers)
	prometheus.MustRegister(fnImages)
}

func NewStats() *stats {
	return &stats{
		containers: make(map[string]float64),
		images:     make(map[string]float64),
	}
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

func (s *stats) Containers(containers []ContainerPair) {
	if len(containers) == 0 {
		return
	}
	s.mu.Lock()
	for _, pair := range containers {
		s.containers[pair.state] = pair.value
		fnContainers.WithLabelValues(pair.state).Set(pair.value)
	}
	s.mu.Unlock()
}

func (s *stats) Images(images []ImagePair) {
	if len(images) == 0 {
		return
	}
	s.mu.Lock()
	for _, pair := range images {
		s.images[pair.state] = pair.value
		fnImages.WithLabelValues(pair.state).Set(pair.value)
	}
	s.mu.Unlock()
}

func (s *stats) Enqueue(app string, path string) {
	s.mu.Lock()

	s.queue++
	s.getStatsForFunction(path).queue++
	fnQueued.WithLabelValues(app, path).Inc()

	s.mu.Unlock()
}

// Call when a function has been queued but cannot be started because of an error
func (s *stats) Dequeue(app string, path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	fnQueued.WithLabelValues(app, path).Dec()

	s.mu.Unlock()
}

func (s *stats) DequeueAndStart(app string, path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	fnQueued.WithLabelValues(app, path).Dec()

	s.running++
	s.getStatsForFunction(path).running++
	fnRunning.WithLabelValues(app, path).Inc()

	s.mu.Unlock()
}

func (s *stats) Complete(app string, path string) {
	s.mu.Lock()

	s.running--
	s.getStatsForFunction(path).running--
	fnRunning.WithLabelValues(app, path).Dec()

	s.complete++
	s.getStatsForFunction(path).complete++
	fnCompleted.WithLabelValues(app, path).Inc()

	s.mu.Unlock()
}

func (s *stats) Failed(app string, path string) {
	s.mu.Lock()

	s.running--
	s.getStatsForFunction(path).running--
	fnRunning.WithLabelValues(app, path).Dec()

	s.failed++
	s.getStatsForFunction(path).failed++
	fnFailed.WithLabelValues(app, path).Inc()

	s.mu.Unlock()
}

func (s *stats) DequeueAndFail(app string, path string) {
	s.mu.Lock()

	s.queue--
	s.getStatsForFunction(path).queue--
	fnQueued.WithLabelValues(app, path).Dec()

	s.failed++
	s.getStatsForFunction(path).failed++
	fnFailed.WithLabelValues(app, path).Inc()

	s.mu.Unlock()
}

func (s *stats) Stats() Stats {
	var stats Stats
	s.mu.Lock()
	stats.Running = s.running
	stats.Complete = s.complete
	stats.Queue = s.queue
	stats.Failed = s.failed

	stats.Containers = make(map[string]float64)
	for key, value := range s.containers {
		stats.Containers[key] = value
	}

	stats.Images = make(map[string]float64)
	for key, value := range s.images {
		stats.Images[key] = value
	}

	stats.FunctionStatsMap = make(map[string]*FunctionStats)
	for key, value := range s.functionStatsMap {
		thisFunctionStats := &FunctionStats{Queue: value.queue, Running: value.running, Complete: value.complete, Failed: value.failed}
		stats.FunctionStatsMap[key] = thisFunctionStats
	}
	s.mu.Unlock()
	return stats
}
