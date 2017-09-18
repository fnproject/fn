package agent

import "sync"

// TODO this should expose:
// * hot containers active
// * memory used / available

// global statistics
type stats struct {
	mu       sync.Mutex
    // statistics for all functions combined
	queue    uint64
	running  uint64
	complete uint64
	// statistics for individual functions, keyed by function path
	functionStatsMap map[string]*functionStats
}

// statistics for an individual function
type functionStats struct {
	queue    uint64
    running  uint64
    complete uint64
}

type Stats struct {
    // statistics for all functions combined
	Queue    uint64
	Running  uint64
	Complete uint64
	// statistics for individual functions, keyed by function path
	FunctionStatsMap map[string]*FunctionStats	
}

// statistics for an individual function
type FunctionStats struct {
	Queue    uint64
	Running  uint64
	Complete uint64
}

func (s *stats) getStatsForFunction(path string) *functionStats {
	if s.functionStatsMap == nil {
    	s.functionStatsMap = make(map[string]*functionStats)
    }
    thisFunctionStats, ok := s.functionStatsMap[path]
    if !ok {
    	thisFunctionStats = &functionStats{}
    }
    
	s.functionStatsMap[path]=thisFunctionStats
	return thisFunctionStats
}

func (s *stats) Enqueue(path string) {
	s.mu.Lock()
	s.queue++
	s.getStatsForFunction(path).queue++
	s.mu.Unlock()
}

// Call when a function has been queued but cannot be started because of an error
func (s *stats) Dequeue(path string) {
	s.mu.Lock()
	s.queue--
	s.getStatsForFunction(path).queue--
	s.mu.Unlock()
}

func (s *stats) Start(path string) {
	s.mu.Lock()
	s.queue--
	s.getStatsForFunction(path).queue--
	s.running++
	s.getStatsForFunction(path).running++
	s.mu.Unlock()
}

func (s *stats) Complete(path string) {
	s.mu.Lock()
	s.running--
	s.getStatsForFunction(path).running--
	s.complete++
	s.getStatsForFunction(path).complete++
	s.mu.Unlock()
}

func (s *stats) Stats() Stats {
	var stats Stats
	s.mu.Lock()
	stats.Running = s.running
	stats.Complete = s.complete
	stats.Queue = s.queue
	stats.FunctionStatsMap = make(map[string]*FunctionStats)
	for key,value := range s.functionStatsMap {
		thisFunctionStats := &FunctionStats{Queue: value.queue, Running: value.running, Complete: value.complete}
		stats.FunctionStatsMap[key]=thisFunctionStats
	}
	s.mu.Unlock()
	return stats
}
