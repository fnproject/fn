package grpc

import (
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/poolmanager"
)

// allow factory to be overridden in tests
type insecureRunnerFactory func(addr string) (agent.Runner, error)

func insecureGRPCRunnerFactory(addr string) (agent.Runner, error) {
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
type staticNodePool struct {
	generator insecureRunnerFactory
	rMtx      *sync.RWMutex
	runners   []agent.Runner
}

// NewStaticNodePool returns a NodePool consisting of a static set of runners
func DefaultStaticNodePool(runnerAddresses []string) agent.NodePool {
	return newStaticNodePool(runnerAddresses, insecureGRPCRunnerFactory)
}

// NewStaticNodePool returns a NodePool consisting of a static set of runners
func newStaticNodePool(runnerAddresses []string, runnerFactory insecureRunnerFactory) agent.NodePool {
	logrus.WithField("runners", runnerAddresses).Info("Starting static runner pool")
	var runners []agent.Runner
	for _, addr := range runnerAddresses {
		r, err := runnerFactory(addr)
		if err != nil {
			logrus.WithField("runner_addr", addr).Warn("Invalid runner")
			continue
		}
		logrus.WithField("runner_addr", addr).Debug("Adding runner to pool")
		runners = append(runners, r)
	}
	return &staticNodePool{
		rMtx:      &sync.RWMutex{},
		runners:   runners,
		generator: runnerFactory,
	}
}

func (np *staticNodePool) Runners(lbgID string) []agent.Runner {
	np.rMtx.RLock()
	defer np.rMtx.RUnlock()

	r := make([]agent.Runner, len(np.runners))
	copy(r, np.runners)
	return r
}

func (np *staticNodePool) AddRunner(address string) error {
	np.rMtx.Lock()
	defer np.rMtx.Unlock()

	// don't add duplicates
	for _, r := range np.runners {
		if r.Address() == address {
			return nil
		}
	}

	r, err := np.generator(address)
	if err != nil {
		logrus.WithField("runner_addr", address).Warn("Failed to add runner")
		return err
	}
	np.runners = append(np.runners, r)
	return nil
}

func (np *staticNodePool) RemoveRunner(address string) {
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

func (np *staticNodePool) AssignCapacity(r *poolmanager.CapacityRequest) {
	// NO-OP
}

func (np *staticNodePool) ReleaseCapacity(r *poolmanager.CapacityRequest) {
	// NO-OP
}

func (np *staticNodePool) Shutdown() {
	// NO-OP
}
