package supervisor

import (
	"context"
	"sync"
)

var (
	defaultSupervisor Supervisor
	running           sync.Mutex
	defaultContext    = context.Background()
)

func init() {
	defaultSupervisor.Name = "default supervisor"
}

// Add inserts new service into the default supervisor. If it is already
// started, it will launch it automatically.
func Add(service Service) {
	defaultSupervisor.Add(service)
}

// AddFunc inserts new anonymous service into the default supervisor. If it is
// already started, it will launch it automatically.
func AddFunc(f func(context.Context)) {
	defaultSupervisor.AddFunc(f)
}

// Cancelations return a list of services names of default supervisor and their
// cancelation calls. These calls be used to force a service restart.
func Cancelations() map[string]context.CancelFunc {
	return defaultSupervisor.Cancelations()
}

// Remove stops the service in the default supervisor tree and remove from it.
func Remove(name string) {
	defaultSupervisor.Remove(name)
}

// Serve starts the default supervisor tree. It can be started only once at a
// time. If stopped (canceled), it can be restarted. In case of concurrent
// calls, it will hang until the current call is completed. It can run only one
// per package-level. If you need many, use
// supervisor.Supervisor/supervisor.Group instead of supervisor.Serve{,Group}.
// After its conclusion, its internal state is reset.
func Serve() {
	running.Lock()
	defaultSupervisor.Serve(defaultContext)
	defaultSupervisor.reset()
	defaultContext = context.Background()
	running.Unlock()
}

// ServeContext starts the default upervisor tree with a custom context.Context.
// It can be started only once at a time. If stopped (canceled), it can be
// restarted. In case of concurrent calls, it will hang until the current call
// is completed. After its conclusion, its internal state is reset.
func ServeContext(ctx context.Context) {
	running.Lock()
	defaultSupervisor.Serve(ctx)
	defaultSupervisor.reset()
	running.Unlock()
}

// ServeGroup starts the default supervisor tree within a Group. It can be
// started only once at a time. If stopped (canceled), it can be restarted.
// In case of concurrent calls, it will hang until the current call is
// completed.  It can run only one per package-level. If you need many, use
// supervisor.ServeContext/supervisor.ServeGroupContext instead of
// supervisor.Serve/supervisor.ServeGroup. After its conclusion, its internal
// state is reset.
func ServeGroup() {
	running.Lock()
	var group Group
	group.Supervisor = &defaultSupervisor
	group.Serve(defaultContext)
	defaultSupervisor.reset()
	defaultContext = context.Background()
	running.Unlock()
}

// ServeGroupContext starts the defaultSupervisor tree with a custom
// context.Context. It can be started only once at a time. If stopped
// (canceled), it can be restarted. In case of concurrent calls, it will hang
// until the current call is completed. After its conclusion, its internal
// state is reset.
func ServeGroupContext(ctx context.Context) {
	running.Lock()
	var group Group
	group.Supervisor = &defaultSupervisor
	group.Serve(ctx)
	defaultSupervisor.reset()
	running.Unlock()
}

// Services return a list of services of default supervisor.
func Services() map[string]Service {
	return defaultSupervisor.Services()
}

// SetDefaultContext allows to change the context used for supervisor.Serve()
// and supervisor.ServeGroup().
func SetDefaultContext(ctx context.Context) {
	running.Lock()
	defaultContext = ctx
	running.Unlock()
}
