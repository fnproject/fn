package runner

import "sync"

type stats struct {
	mu       sync.Mutex
	queue    uint64
	running  uint64
	complete uint64
}

type Stats struct {
	Queue    uint64
	Running  uint64
	Complete uint64
}

func (s *stats) Enqueue() {
	s.mu.Lock()
	s.queue++
	s.mu.Unlock()
}

func (s *stats) Start() {
	s.mu.Lock()
	s.queue--
	s.running++
	s.mu.Unlock()
}

func (s *stats) Complete() {
	s.mu.Lock()
	s.running--
	s.complete++
	s.mu.Unlock()
}

func (s *stats) Stats() Stats {
	var stats Stats
	s.mu.Lock()
	stats.Running = s.running
	stats.Complete = s.complete
	stats.Queue = s.queue
	s.mu.Unlock()
	return stats
}
