package agent

import (
	"context"
	"crypto/tls"

	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/sirupsen/logrus"
)

// manages a single set of runners ignoring lb groups
type staticRunnerPool struct {
	generator pool.MTLSRunnerFactory
	tlsConf   *tls.Config // can be nil when running in insecure mode
	runnerCN  string
	runners   []pool.Runner
}

func DefaultStaticRunnerPool(runnerAddresses []string) pool.RunnerPool {
	return NewStaticRunnerPool(runnerAddresses, nil, SecureGRPCRunnerFactory)
}

func NewStaticRunnerPool(runnerAddresses []string, tlsConf *tls.Config, runnerFactory pool.MTLSRunnerFactory) pool.RunnerPool {
	logrus.WithField("runners", runnerAddresses).Info("Starting static runner pool")
	var runners []pool.Runner
	for _, addr := range runnerAddresses {
		r, err := runnerFactory(addr, tlsConf)
		if err != nil {
			logrus.WithError(err).WithField("runner_addr", addr).Warn("Invalid runner")
			continue
		}
		logrus.WithField("runner_addr", addr).Debug("Adding runner to pool")
		runners = append(runners, r)
	}
	return &staticRunnerPool{
		runners:   runners,
		tlsConf:   tlsConf,
		generator: runnerFactory,
	}
}

func (rp *staticRunnerPool) Runners(ctx context.Context, call pool.RunnerCall) ([]pool.Runner, error) {
	r := make([]pool.Runner, len(rp.runners))
	copy(r, rp.runners)
	return r, nil
}

func (rp *staticRunnerPool) Shutdown(ctx context.Context) error {
	var retErr error
	for _, r := range rp.runners {
		err := r.Close(ctx)
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
