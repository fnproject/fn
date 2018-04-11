package agent

import (
	"context"
	"testing"

	pool "github.com/fnproject/fn/api/runnerpool"
)

func setupStaticPool(runners []string) pool.RunnerPool {
	return NewStaticRunnerPool(runners, nil, "", mockRunnerFactory)
}

type mockStaticRunner struct {
	address string
}

func (r *mockStaticRunner) TryExec(ctx context.Context, call pool.RunnerCall) (bool, error) {
	return true, nil
}

func (r *mockStaticRunner) Close() error {
	return nil
}

func (r *mockStaticRunner) Address() string {
	return r.address
}

func mockRunnerFactory(addr, cn string, pki *pool.PKIData) (pool.Runner, error) {
	return &mockStaticRunner{address: addr}, nil
}

func TestNewStaticPool(t *testing.T) {
	addrs := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	np := setupStaticPool(addrs)

	runners, err := np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != len(addrs) {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}
}

func TestEmptyPool(t *testing.T) {
	np := setupStaticPool(nil).(*staticRunnerPool)

	runners, err := np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	np.AddRunner("127.0.0.1:8082")

	runners, err = np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	np.Shutdown()

	runners, err = np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

}

func TestAddNodeToPool(t *testing.T) {
	addrs := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	np := setupStaticPool(addrs).(*staticRunnerPool)

	np.AddRunner("127.0.0.1:8082")
	np.AddRunner("127.0.0.1:8082")

	runners, err := np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 3 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	np.Shutdown()

	runners, err = np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}
}

func TestRemoveNodeFromPool(t *testing.T) {
	addrs := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	np := setupStaticPool(addrs).(*staticRunnerPool)

	np.RemoveRunner("127.0.0.1:8081")

	runners, err := np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}

	if len(runners) != 1 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	np.RemoveRunner("127.0.0.1:8081")

	runners, err = np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 1 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	np.RemoveRunner("127.0.0.1:8080")

	runners, err = np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	np.RemoveRunner("127.0.0.1:8080")

	runners, err = np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	np.Shutdown()

	runners, err = np.Runners(nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != 0 {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

}
