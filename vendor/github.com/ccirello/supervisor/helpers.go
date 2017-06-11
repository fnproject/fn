package supervisor

import (
	"context"
	"fmt"
	"sync"
)

func serve(s *Supervisor, ctx context.Context, processFailure processFailure) {
	s.running.Lock()
	defer s.running.Unlock()

	startServices(s, ctx, processFailure)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for {
			select {
			case <-s.added:
				startServices(s, ctx, processFailure)

			case <-ctx.Done():
				wg.Done()
				return
			}
		}
	}()
	<-ctx.Done()

	wg.Wait()
	s.runningServices.Wait()

	s.mu.Lock()
	s.cancelations = make(map[string]context.CancelFunc)
	s.mu.Unlock()
}

func startServices(s *Supervisor, supervisorCtx context.Context, processFailure processFailure) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup
	for _, name := range s.svcorder {
		svc := s.services[name]
		if _, ok := s.cancelations[name]; ok {
			continue
		}

		wg.Add(1)

		terminateCtx, terminate := context.WithCancel(supervisorCtx)
		s.cancelations[name] = terminate
		s.terminations[name] = terminate

		go func(name string, svc service) {
			s.runningServices.Add(1)
			defer s.runningServices.Done()
			wg.Done()
			retry := true
			for retry {
				retry = svc.svctype == Permanent
				s.log(fmt.Sprintf("%s starting", name))
				func() {
					defer func() {
						if r := recover(); r != nil {
							s.log(fmt.Sprintf("%s panic: %v", name, r))
							retry = svc.svctype == Permanent || svc.svctype == Transient
						}
					}()
					ctx, cancel := context.WithCancel(terminateCtx)
					s.mu.Lock()
					s.cancelations[name] = cancel
					s.mu.Unlock()
					svc.svc.Serve(ctx)
				}()
				if retry {
					processFailure()
				}
				select {
				case <-terminateCtx.Done():
					s.log(fmt.Sprintf("%s restart aborted (terminated)", name))
					return
				case <-supervisorCtx.Done():
					s.log(fmt.Sprintf("%s restart aborted (supervisor halted)", name))
					return
				default:
				}
				switch svc.svctype {
				case Temporary:
					s.log(fmt.Sprintf("%s exited (temporary)", name))
					return
				case Transient:
					s.log(fmt.Sprintf("%s exited (transient)", name))
				default:
					s.log(fmt.Sprintf("%s exited (permanent)", name))
				}
			}
		}(name, svc)
	}
	wg.Wait()
}
