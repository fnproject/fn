package supervisor

import (
	"context"
	"sync"
)

// Group is a superset of Supervisor datastructure responsible for offering a
// supervisor tree whose all services are restarted whenever one of them fail or
// is restarted. It assumes that all services rely on each other. It implements
// Service, therefore it can be nested if necessary either with other Group or
// Supervisor. When passing the Group around, remind to do it as reference
// (&group).
type Group struct {
	*Supervisor
}

// Serve starts the Group tree. It can be started only once at a time. If
// stopped (canceled), it can be restarted. In case of concurrent calls, it will
// hang until the current call is completed.
func (g *Group) Serve(ctx context.Context) {
	if g.Supervisor == nil {
		panic("Supervisor missing for this Group.")
	}
	g.Supervisor.prepare()
	restartCtx, cancel := context.WithCancel(ctx)

	var (
		mu                sync.Mutex
		processingFailure bool
	)
	processFailure := func() {
		mu.Lock()
		if processingFailure {
			mu.Unlock()
			return
		}
		processingFailure = true
		mu.Unlock()

		if !g.shouldRestart() {
			cancel()
			return
		}

		g.mu.Lock()
		g.log("halting all services after failure")
		for _, c := range g.terminations {
			c()
		}

		g.cancelations = make(map[string]context.CancelFunc)
		g.mu.Unlock()

		go func() {
			g.log("waiting for all services termination")
			g.runningServices.Wait()
			g.log("waiting for all services termination - completed")

			mu.Lock()
			processingFailure = false
			mu.Unlock()

			g.log("triggering group restart")
			g.added <- struct{}{}
		}()
	}
	serve(g.Supervisor, restartCtx, processFailure)
}
