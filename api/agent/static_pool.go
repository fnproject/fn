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

func (rp *staticRunnerPool) shutdown() []pool.Runner {
	rp.rMtx.Lock()
	defer rp.rMtx.Unlock()

	if rp.isClosed {
		return nil
	}

	rp.isClosed = true
	toRemove := rp.runners[:]
	rp.runners = nil

	return toRemove
}

func (rp *staticRunnerPool) addRunner(runner pool.Runner) error {
	rp.rMtx.Lock()
	defer rp.rMtx.Unlock()

	if rp.isClosed {
		return ErrorPoolClosed
	}

	isFound := false
	for _, r := range rp.runners {
		if r.Address() == runner.Address() {
			isFound = true
			break
		}
	}
	if !isFound {
		rp.runners = append(rp.runners, runner)
	}

	return nil
}

func (rp *staticRunnerPool) removeRunner(address string) pool.Runner {
	rp.rMtx.Lock()
	defer rp.rMtx.Unlock()

	for i, r := range rp.runners {
		if r.Address() == address {
			rp.runners = append(rp.runners[:i], rp.runners[i+1:]...)
			return r
		}
	}
	return nil
}

func (rp *staticRunnerPool) getRunners() ([]pool.Runner, error) {
	rp.rMtx.RLock()
	defer rp.rMtx.RUnlock()

	if rp.isClosed {
		return nil, ErrorPoolClosed
	}

	r := make([]pool.Runner, len(rp.runners))
	copy(r, rp.runners)

	return r, nil
}

func (rp *staticRunnerPool) Runners(call pool.RunnerCall) ([]pool.Runner, error) {
	return rp.getRunners()
}

func (rp *staticRunnerPool) AddRunner(address string) error {
	r, err := rp.generator(address, rp.runnerCN, rp.pki)
	if err != nil {
		logrus.WithField("runner_addr", address).Warn("Failed to add runner")
		return err
	}

	err = rp.addRunner(r)
	if err != nil {
		r.Close()
	}
	return err
}

func (rp *staticRunnerPool) RemoveRunner(address string) {
	toRemove := rp.removeRunner(address)
	if toRemove == nil {
		return
	}

	err := toRemove.Close()
	if err != nil {
		logrus.WithError(err).WithField("runner_addr", toRemove.Address()).Error("Error closing runner")
	}
}

func (rp *staticRunnerPool) Shutdown() error {
	toRemove := rp.shutdown()

	var retErr error
	for _, r := range toRemove {
		err := r.Close()
		if err != nil {
			logrus.WithError(err).WithField("runner_addr", r.Address()).Error("Error closing runner")
			// grab the first error only for now.
			if retErr == nil {
				retErr = err
			}
		}
	}
	return retErr
}
