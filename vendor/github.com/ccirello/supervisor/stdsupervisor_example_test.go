package supervisor_test

import (
	"context"
	"fmt"
	"sync"

	"cirello.io/supervisor"
)

type Simpleservice struct {
	id int
	sync.WaitGroup
}

func (s *Simpleservice) Serve(ctx context.Context) {
	fmt.Println(s.String())
	s.Done()
	<-ctx.Done()
}

func (s *Simpleservice) String() string {
	return fmt.Sprintf("simple service %d", s.id)
}

func ExampleAddFunc() {
	var svc sync.WaitGroup

	svc.Add(1)
	supervisor.AddFunc(func(ctx context.Context) {
		fmt.Println("anonymous service")
		svc.Done()
		<-ctx.Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.ServeContext(ctx)
		wg.Done()
	}()

	svc.Wait()
	cancel()
	wg.Wait()

	// output:
	// anonymous service
}

func ExampleServeContext() {
	svc := &Simpleservice{id: 1}
	svc.Add(1)
	supervisor.Add(svc)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.ServeContext(ctx)
		wg.Done()
	}()

	svc.Wait()
	cancel()
	wg.Wait()

	// output:
	// simple service 1
}

func ExampleServeGroupContext() {
	svc1 := &Simpleservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &Simpleservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.ServeGroupContext(ctx)
		wg.Done()
	}()

	svc1.Wait()
	svc2.Wait()
	cancel()
	wg.Wait()

	// unordered output:
	// simple service 1
	// simple service 2
}

func ExampleServe() {
	svc := &Simpleservice{id: 1}
	svc.Add(1)
	supervisor.Add(svc)

	var cancel context.CancelFunc
	ctx, cancel := context.WithCancel(context.Background())
	supervisor.SetDefaultContext(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.Serve()
		wg.Done()
	}()

	svc.Wait()
	cancel()
	wg.Wait()

	// output:
	// simple service 1
}

func ExampleServeGroup() {
	svc1 := &Simpleservice{id: 1}
	svc1.Add(1)
	supervisor.Add(svc1)
	svc2 := &Simpleservice{id: 2}
	svc2.Add(1)
	supervisor.Add(svc2)

	ctx, cancel := context.WithCancel(context.Background())
	supervisor.SetDefaultContext(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		supervisor.ServeGroup()
		wg.Done()
	}()

	svc1.Wait()
	svc2.Wait()
	cancel()
	wg.Wait()

	// unordered output:
	// simple service 1
	// simple service 2
}
