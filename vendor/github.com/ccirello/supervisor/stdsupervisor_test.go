package supervisor

import (
	"context"
	"sync"
	"testing"
)

func TestAddFunc(t *testing.T) {
	t.Parallel()

	universalAnonSvcMu.Lock()
	oldCount := universalAnonSvc
	universalAnonSvc = 0
	universalAnonSvcMu.Unlock()
	defer func() {
		universalAnonSvcMu.Lock()
		universalAnonSvc = oldCount
		universalAnonSvcMu.Unlock()
	}()

	var (
		runCount int
		wg       sync.WaitGroup
	)

	wg.Add(1)
	AddFunc(func(ctx context.Context) {
		runCount++
		wg.Done()
		<-ctx.Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	go ServeContext(ctx)

	svcs := Services()
	if _, ok := svcs["anonymous service 1"]; !ok {
		t.Errorf("anonymous service was not found in service list")
	}

	wg.Wait()
	cancel()

	if runCount == 0 {
		t.Errorf("anonymous service should have been started")
	}
}

func TestDefaultSupevisorAndGroup(t *testing.T) {
	t.Parallel()

	svc := &holdingservice{id: 1}
	svc.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	SetDefaultContext(ctx)
	Add(svc)
	if len(defaultSupervisor.services) != 1 {
		t.Errorf("%s should have been added", svc.String())
	}

	Remove(svc.String())
	if len(defaultSupervisor.services) != 0 {
		t.Errorf("%s should have been removed. services: %#v", svc.String(), defaultSupervisor.services)
	}

	Add(svc)

	svcs := Services()
	if _, ok := svcs[svc.String()]; !ok {
		t.Errorf("%s should have been found", svc.String())
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		Serve()
		wg.Done()
	}()

	svc.Wait()

	cs := Cancelations()
	if _, ok := cs[svc.String()]; !ok {
		t.Errorf("%s's cancelation should have been found. %#v", svc.String(), cs)
	}

	cancel()
	wg.Wait()

	ctx, cancel = context.WithCancel(context.Background())
	SetDefaultContext(ctx)
	svc.Add(1)
	Add(svc)
	if len(defaultSupervisor.services) != 1 {
		t.Errorf("%s should have been added", svc.String())
	}

	wg.Add(1)
	go func() {
		ServeGroup()
		wg.Done()
	}()

	svc.Wait()
	cancel()
	wg.Wait()
}
