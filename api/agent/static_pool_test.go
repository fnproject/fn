package agent

import (
	"context"
	"crypto/tls"
	"errors"
	"testing"

	pool "github.com/fnproject/fn/api/runnerpool"
)

func setupStaticPool(runners []string) pool.RunnerPool {
	return NewStaticRunnerPool(runners, nil, mockRunnerFactory)
}

var (
	ErrorGarbanzoBeans = errors.New("yes, that's right. Garbanzo beans...")
)

type mockStaticRunner struct {
	address string
}

func (r *mockStaticRunner) TryExec(ctx context.Context, call pool.RunnerCall) (bool, error) {
	return true, nil
}

func (r *mockStaticRunner) Status(ctx context.Context) (*pool.RunnerStatus, error) {
	return nil, nil
}

func (r *mockStaticRunner) Close(context.Context) error {
	return ErrorGarbanzoBeans
}

func (r *mockStaticRunner) Address() string {
	return r.address
}

func mockRunnerFactory(addr string, tlsConf *tls.Config) (pool.Runner, error) {
	return &mockStaticRunner{address: addr}, nil
}

func TestNewStaticPool(t *testing.T) {
	addrs := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	np := setupStaticPool(addrs)

	runners, err := np.Runners(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != len(addrs) {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	err = np.Shutdown(context.Background())
	if err != ErrorGarbanzoBeans {
		t.Fatalf("Expected garbanzo beans error from shutdown %v", err)
	}
}

func TestEmptyPool(t *testing.T) {
	np := setupStaticPool(nil).(*staticRunnerPool)

	runners, err := np.Runners(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	err = np.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error from shutdown %v", err)
	}
}
