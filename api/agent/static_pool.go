package agent

import (
	"sync"

	"github.com/fnproject/fn/api/models"
	"github.com/sirupsen/logrus"
)

// allow factory to be overridden in tests
type insecureRunnerFactory func(addr string) (models.Runner, error)

func insecureGRPCRunnerFactory(addr string) (models.Runner, error) {
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
	runners   []models.Runner
}

// DefaultStaticRunnerPool returns a RunnerPool consisting of a static set of runners
func DefaultStaticRunnerPool(runnerAddresses []string) models.RunnerPool {
	return newStaticRunnerPool(runnerAddresses, insecureGRPCRunnerFactory)
}

func newStaticRunnerPool(runnerAddresses []string, runnerFactory insecureRunnerFactory) models.RunnerPool {
	logrus.WithField("runners", runnerAddresses).Info("Starting static runner pool")
	var runners []models.Runner
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

func (np *staticRunnerPool) Runners(call models.RunnerCall) []models.Runner {
	np.rMtx.RLock()
	defer np.rMtx.RUnlock()

	r := make([]models.Runner, len(np.runners))
	copy(r, np.runners)
	return r
}

func (np *staticRunnerPool) AddRunner(address string) error {
	np.rMtx.Lock()
	defer np.rMtx.Unlock()

	r, err := np.generator(address)
	if err != nil {
		logrus.WithField("runner_addr", address).Warn("Failed to add runner")
		return err
	}
	np.runners = append(np.runners, r)
	return nil
}

func (np *staticRunnerPool) RemoveRunner(address string) {
	np.rMtx.Lock()
	defer np.rMtx.Unlock()

	for i, r := range np.runners {
		if r.Address() == address {
			// delete runner from list
			np.runners = append(np.runners[:i], np.runners[i+1:]...)
			return
		}
	}
}

func (np *staticRunnerPool) Shutdown() {
	// NO-OP
}
