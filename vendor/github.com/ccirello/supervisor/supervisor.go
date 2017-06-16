package supervisor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type service struct {
	svc     Service
	svctype ServiceType
}

type processFailure func()

// AlwaysRestart adjusts the supervisor to never halt in face of failures.
const AlwaysRestart = -1

// ServiceType defines the restart strategy for a service.
type ServiceType int

const (
	// Permanent services are always restarted
	Permanent ServiceType = iota
	// Transient services are restarted only when panic.
	Transient
	// Temporary services are never restarted.
	Temporary
)

// Service is the public interface expected by a Supervisor.
//
// This will be internally named after the result of fmt.Stringer, if available.
// Otherwise it is going to use an internal representation for the service
// name.
type Service interface {
	// Serve is called by a Supervisor to start the service. It expects the
	// service to honor the passed context and its lifetime. Observe
	// <-ctx.Done() and ctx.Err(). If the service is stopped by anything
	// but the Supervisor, it will get started again. Be careful with shared
	// state among restarts.
	Serve(ctx context.Context)
}

// Supervisor is the basic datastructure responsible for offering a supervisor
// tree. It implements Service, therefore it can be nested if necessary. When
// passing the Supervisor around, remind to do it as reference (&supervisor).
// Once the supervisor is started, its attributes are frozen.
type Supervisor struct {
	// Name for this supervisor tree, used for logging.
	Name string
	name string

	// MaxRestarts is the number of maximum restarts given MaxTime. If more
	// than MaxRestarts occur in the last MaxTime, then the supervisor
	// stops all services and halts. Set this to AlwaysRestart to prevent
	// supervisor halt.
	MaxRestarts int
	maxrestarts int


	// MaxTime is the time period on which the internal restart count will
	// be reset.
	MaxTime time.Duration
	maxtime time.Duration

	// Log is a replaceable function used for overall logging.
	// Default: log.Printf.
	Log func(interface{})
	log func(interface{})

	// indicates that supervisor is ready for use.
	prepared sync.Once

	// signals that a new service has just been added, so the started
	// supervisor picks it up.
	added chan struct{}

	// indicates that supervisor has running services.
	running         sync.Mutex
	runningServices sync.WaitGroup

	mu           sync.Mutex
	svcorder     []string                      // order in which services must be started
	services     map[string]service            // added services
	cancelations map[string]context.CancelFunc // each service cancelation
	terminations map[string]context.CancelFunc // each service termination call
	lastRestart  time.Time
	restarts     int
}

func (s *Supervisor) prepare() {
	s.prepared.Do(s.reset)
}

func (s *Supervisor) reset() {
	s.mu.Lock()
	if s.Name == "" {
		s.Name = "supervisor"
	}
	if s.MaxRestarts == 0 {
		s.MaxRestarts = 5
	}
	if s.MaxTime == 0 {
		s.MaxTime = 15 * time.Second
	}
	if s.Log == nil {
		s.Log = func(msg interface{}) {
			log.Printf("%s: %v", s.Name, msg)
		}
	}

	s.name = s.Name
	s.maxrestarts = s.MaxRestarts
	s.maxtime = s.MaxTime
	s.log = s.Log

	s.added = make(chan struct{})
	s.cancelations = make(map[string]context.CancelFunc)
	s.services = make(map[string]service)
	s.terminations = make(map[string]context.CancelFunc)
	s.mu.Unlock()
}

func (s *Supervisor) shouldRestart() bool {
	if s.maxrestarts == AlwaysRestart {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastRestart) > s.maxtime {
		s.restarts = 0
	}
	s.lastRestart = time.Now()
	s.restarts++
	return s.restarts < s.maxrestarts
}

// Cancelations return a list of services names and their cancelation calls.
// These calls be used to force a service restart.
func (s *Supervisor) Cancelations() map[string]context.CancelFunc {
	svclist := make(map[string]context.CancelFunc)
	s.mu.Lock()
	for k, v := range s.cancelations {
		svclist[k] = v
	}
	s.mu.Unlock()
	return svclist
}

// Add inserts into the Supervisor tree a new permanent service. If the
// Supervisor is already started, it will start it automatically.
func (s *Supervisor) Add(service Service) {
	s.AddService(service, Permanent)
}

// AddFunc inserts into the Supervisor tree a new permanent anonymous service.
// If the Supervisor is already started, it will start it automatically.
func (s *Supervisor) AddFunc(f func(context.Context)) {
	s.AddService(newAnonymousService(f), Permanent)
}

// AddService inserts into the Supervisor tree a new service of ServiceType. If
// the Supervisor is already started, it will start it automatically. If the
// same service is added more than once, it will reset its backoff mechanism and
// force a service restart.
func (s *Supervisor) AddService(svc Service, svctype ServiceType) {
	s.prepare()

	name := fmt.Sprintf("%s", svc)
	s.mu.Lock()
	s.services[name] = service{
		svc:     svc,
		svctype: svctype,
	}
	s.svcorder = append(s.svcorder, name)
	s.mu.Unlock()

	go func() {
		s.added <- struct{}{}
	}()
}

// Remove stops the service in the Supervisor tree and remove from it.
func (s *Supervisor) Remove(name string) {
	s.prepare()

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.services[name]; !ok {
		return
	}

	delete(s.services, name)

	for i, n := range s.svcorder {
		if name == n {
			s.svcorder = append(s.svcorder[:i], s.svcorder[i+1:]...)
			break
		}
	}

	if c, ok := s.terminations[name]; ok {
		delete(s.terminations, name)
		c()
	}

	if _, ok := s.cancelations[name]; ok {
		delete(s.cancelations, name)
	}
}

// Serve starts the Supervisor tree. It can be started only once at a time. If
// stopped (canceled), it can be restarted. In case of concurrent calls, it will
// hang until the current call is completed.
func (s *Supervisor) Serve(ctx context.Context) {
	s.prepare()
	restartCtx, cancel := context.WithCancel(ctx)
	processFailure := func() {
		restart := s.shouldRestart()
		if !restart {
			cancel()
		}
	}
	serve(s, restartCtx, processFailure)
}

// Services return a list of services
func (s *Supervisor) Services() map[string]Service {
	svclist := make(map[string]Service)
	s.mu.Lock()
	for k, v := range s.services {
		svclist[k] = v.svc
	}
	s.mu.Unlock()
	return svclist
}

func (s *Supervisor) String() string {
	s.prepare()
	return s.name
}
