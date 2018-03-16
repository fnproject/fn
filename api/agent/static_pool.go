package agent

import (
	"context"
	"sync"
	"time"

	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/sirupsen/logrus"
)

const (
	staticPoolShutdownTimeout = 5 * time.Second
)

// allow factory to be overridden in tests
type insecureRunnerFactory func(addr string) (pool.Runner, error)

func insecureGRPCRunnerFactory(addr string) (pool.Runner, error) {
	conn, client, err := runnerConnection(addr, nil)
	if err != nil {
		return nil, err
	}

	return &gRPCRunner{
		address: addr,
		conn:    conn,
		client:  client,
	}, nil
}

// manages a single set of runners ignoring lb groups
type staticRunnerPool struct {
	generator insecureRunnerFactory
	rMtx      *sync.RWMutex
	runners   []pool.Runner
}

// DefaultStaticRunnerPool returns a RunnerPool consisting of a static set of runners
func DefaultStaticRunnerPool(runnerAddresses []string) pool.RunnerPool {
	return newStaticRunnerPool(runnerAddresses, insecureGRPCRunnerFactory)
}

func newStaticRunnerPool(runnerAddresses []string, runnerFactory insecureRunnerFactory) pool.RunnerPool {
	logrus.WithField("runners", runnerAddresses).Info("Starting static runner pool")
	var runners []pool.Runner
	for _, addr := range runnerAddresses {
		r, err := runnerFactory(addr)
		if err != nil {
			logrus.WithField("runner_addr", addr).Warn("Invalid runner")
			continue
		}
		logrus.WithField("runner_addr", addr).Debug("Adding runner to pool")
		runners = append(runners, r)
	}
	return &staticRunnerPool{
		rMtx:      &sync.RWMutex{},
		runners:   runners,
		generator: runnerFactory,
	}
}

func (rp *staticRunnerPool) Runners(call pool.RunnerCall) ([]pool.Runner, error) {
	rp.rMtx.RLock()
	defer rp.rMtx.RUnlock()

	r := make([]pool.Runner, len(rp.runners))
	copy(r, rp.runners)
	return r, nil
}

func (rp *staticRunnerPool) AddRunner(address string) error {
	rp.rMtx.Lock()
	defer rp.rMtx.Unlock()

	r, err := rp.generator(address)
	if err != nil {
		logrus.WithField("runner_addr", address).Warn("Failed to add runner")
		return err
	}
	rp.runners = append(rp.runners, r)
	return nil
}

func (rp *staticRunnerPool) RemoveRunner(address string) {
	rp.rMtx.Lock()
	defer rp.rMtx.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), staticPoolShutdownTimeout)
	defer cancel()

	for i, r := range rp.runners {
		if r.Address() == address {
			err := r.Close(ctx)
			if err != nil {
				logrus.WithError(err).WithField("runner_addr", r.Address()).Error("Failed to close runner")
			}
			// delete runner from list
			rp.runners = append(rp.runners[:i], rp.runners[i+1:]...)
			return
		}
	}
}

// Shutdown blocks waiting for all runners to close, or until ctx is done
func (rp *staticRunnerPool) Shutdown(ctx context.Context) (e error) {
	rp.rMtx.Lock()
	defer rp.rMtx.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), staticPoolShutdownTimeout)
	defer cancel()

	errors := make(chan error, len(rp.runners))
	var wg sync.WaitGroup
	for _, r := range rp.runners {
		wg.Add(1)
		go func(runner pool.Runner) {
			defer wg.Done()
			err := runner.Close(ctx)
			if err != nil {
				logrus.WithError(err).WithField("runner_addr", runner.Address()).Error("Failed to close runner")
				errors <- err
			}
		}(r)
	}

	done := make(chan interface{})
	go func() {
		defer close(done)
		wg.Wait()
	}()

	select {
	case <-done:
		close(errors)
		for e := range errors {
			// return the first error
			if e != nil {
				return e
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err() // context timed out while waiting
	}
}
