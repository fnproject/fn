package agent

import (
	"context"
	"github.com/fnproject/fn/api/common"
	"sync"
)

// TODO this should expose:
// * hot containers active
// * memory used / available

// stats is the top-level struct containing composite and individial statistics all routes in all apps
// an instance of this struc is maintained in memory to keep a record of the current stats since the server was started
// access must be synchronized using the Mutex
type stats struct {
	mu sync.Mutex
	// composite statistics for all routes in all apps
	queue    uint64
	running  uint64
	complete uint64
	failed   uint64
	// statistics for individual apps, keyed by appname
	apps map[string]appStats
}

// appStats holds statistics for the routes in an individual app, keyed by the path of the route
// instances of this struc are used to maintain an in-memory record of the current stats
// access must be synchronized using the Mutex on the parent stats
type appStats struct {
	routes map[string]*routeStats
}

// routeStats holds statistics for an individual route
// instances of this struc are used to maintain an in-memory record of the current stats
// access must be synchronized using the Mutex on the parent stats
type routeStats struct {
	queue    uint64
	running  uint64
	complete uint64
	failed   uint64
}

// Stats is the top-level struct containing composite and individial statistics all routes in all apps
// an instance of this struc is created when converting the current stats to JSON
type Stats struct {
	Queue    uint64
	Running  uint64
	Complete uint64
	Failed   uint64
	// statistics for individual apps, keyed by appname
	Apps map[string]AppStats
}

// AppStats holds statistics for the routes in an individual app, keyed by the path of the route
// instances of this struc are used when converting the current stats to JSON
type AppStats struct {
	Routes map[string]*RouteStats
}

// RouteStats holds statistics for an individual route
// instances of this struc are used when converting the current stats to JSON
type RouteStats struct {
	Queue    uint64
	Running  uint64
	Complete uint64
	Failed   uint64
}

// return the stats corresponding to the specified app name and route path, creating a new stats if one does not already exist
func (s *stats) getStatsForRoute(app string, path string) *routeStats {
	if s.apps == nil {
		s.apps = make(map[string]appStats)
	}
	thisAppStats, appFound := s.apps[app]
	if !appFound {
		thisAppStats = appStats{routes: make(map[string]*routeStats)}
		s.apps[app] = thisAppStats
	}
	thisRouteStats, pathFound := thisAppStats.routes[path]
	if !pathFound {
		thisRouteStats = &routeStats{}
		thisAppStats.routes[path] = thisRouteStats
	}
	return thisRouteStats
}

func (s *stats) Enqueue(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForRoute(app, path)
	s.queue++
	fstats.queue++

	s.mu.Unlock()

	common.IncrementGauge(ctx, queuedMetricName)
	common.IncrementCounter(ctx, callsMetricName)
}

// Call when a function has been queued but cannot be started because of an error
func (s *stats) Dequeue(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForRoute(app, path)
	s.queue--
	fstats.queue--

	s.mu.Unlock()

	common.DecrementGauge(ctx, queuedMetricName)
}

func (s *stats) DequeueAndStart(ctx context.Context, app string, path string) {
	s.mu.Lock()

	fstats := s.getStatsForRoute(app, path)
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

	fstats := s.getStatsForRoute(app, path)
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

	fstats := s.getStatsForRoute(app, path)
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

	fstats := s.getStatsForRoute(app, path)
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
	// this creates a Stats from a stats
	// stats is the internal struc which is continuously updated, and access is controlled using its Mutex
	// Stats is a deep copy for external use, and can be converted to JSON
	var stats Stats
	s.mu.Lock()
	stats.Running = s.running
	stats.Complete = s.complete
	stats.Queue = s.queue
	stats.Failed = s.failed
	stats.Apps = make(map[string]AppStats)
	for appname, thisAppStats := range s.apps {
		newAppStats := AppStats{Routes: make(map[string]*RouteStats)}
		stats.Apps[appname] = newAppStats
		for path, thisRouteStats := range thisAppStats.routes {
			newAppStats.Routes[path] = &RouteStats{Queue: thisRouteStats.queue, Running: thisRouteStats.running, Complete: thisRouteStats.complete, Failed: thisRouteStats.failed}
		}
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
