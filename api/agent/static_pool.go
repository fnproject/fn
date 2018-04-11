package agent

import (
	"errors"
	"sync"

	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/sirupsen/logrus"
)

var (
	ErrorPoolClosed = errors.New("Runner pool closed")
)

// manages a single set of runners ignoring lb groups
type staticRunnerPool struct {
	generator pool.MTLSRunnerFactory
	pki       *pool.PKIData // can be nil when running in insecure mode
	runnerCN  string
	rMtx      *sync.RWMutex
	runners   []pool.Runner
	isClosed  bool
}

func DefaultStaticRunnerPool(runnerAddresses []string) pool.RunnerPool {
	return NewStaticRunnerPool(runnerAddresses, nil, "", SecureGRPCRunnerFactory)
}

func NewStaticRunnerPool(runnerAddresses []string, pki *pool.PKIData, runnerCN string, runnerFactory pool.MTLSRunnerFactory) pool.RunnerPool {
	logrus.WithField("runners", runnerAddresses).Info("Starting static runner pool")
	var runners []pool.Runner
	for _, addr := range runnerAddresses {
		r, err := runnerFactory(addr, runnerCN, pki)
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
		pki:       pki,
		runnerCN:  runnerCN,
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
	r, err := rp.generator(address, rp.runnerCN, rp.pki)
	if err != nil {
		logrus.WithField("runner_addr", address).Warn("Failed to add runner")
		return err
	}

	rp.rMtx.Lock()
	if rp.isClosed {
		rp.rMtx.Unlock()
		// this should not block since we have not added it to the pool
		r.Close()
		return ErrorPoolClosed
	}

	isFound := false
	for _, r := range rp.runners {
		if r.Address() == address {
			isFound = true
			break
		}
	}
	if !isFound {
		rp.runners = append(rp.runners, r)
	}

	rp.rMtx.Unlock()
	return nil
}

func (rp *staticRunnerPool) RemoveRunner(address string) {

	var toRemove pool.Runner

	rp.rMtx.Lock()

	for i, r := range rp.runners {
		if r.Address() == address {
			toRemove = r
			rp.runners = append(rp.runners[:i], rp.runners[i+1:]...)
			break
		}
	}

	rp.rMtx.Unlock()

	if toRemove == nil {
		return
	}

	err := toRemove.Close()
	if err != nil {
		logrus.WithError(err).WithField("runner_addr", toRemove.Address()).Error("Error closing runner")
	}
}

func (rp *staticRunnerPool) Shutdown() error {

	rp.rMtx.Lock()
	if rp.isClosed {
		rp.rMtx.Unlock()
		return nil
	}

	rp.isClosed = true
	toRemove := rp.runners[:]
	rp.runners = nil

	rp.rMtx.Unlock()

	var retErr error
	for _, r := range toRemove {
		err := r.Close()
		if err != nil {
			logrus.WithError(err).WithField("runner_addr", r.Address()).Error("Error closing runner")
			retErr = err
		}
	}
	return retErr
}
