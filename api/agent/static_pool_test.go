package agent

import (
	"context"
	"testing"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/models"
)

func setupStaticPool(runners []string) agent.NodePool {
	return newStaticNodePool(runners, mockRunnerFactory)
}

type mockStaticRunner struct {
	address string
}

func (r *mockStaticRunner) TryExec(ctx context.Context, call models.RunnerCall) (bool, error) {
	return true, nil
}

func (r *mockStaticRunner) Close() {

}
func (r *mockStaticRunner) Address() string {
	return r.address
}

func mockRunnerFactory(addr string) (agent.Runner, error) {
	return &mockStaticRunner{address: addr}, nil
}

func TestNewStaticPool(t *testing.T) {
	addrs := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	np := setupStaticPool(addrs)

	if len(np.Runners("foo")) != len(addrs) {
		t.Fatalf("Invalid number of runners %v", len(np.Runners("foo")))
	}
}

func TestAddNodeToPool(t *testing.T) {
	addrs := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	np := setupStaticPool(addrs).(*staticNodePool)
	np.AddRunner("127.0.0.1:8082")

	if len(np.Runners("foo")) != len(addrs)+1 {
		t.Fatalf("Invalid number of runners %v", len(np.Runners("foo")))
	}
}

func TestRemoveNodeFromPool(t *testing.T) {
	addrs := []string{"127.0.0.1:8080", "127.0.0.1:8081"}
	np := setupStaticPool(addrs).(*staticNodePool)
	np.RemoveRunner("127.0.0.1:8081")

	if len(np.Runners("foo")) != len(addrs)-1 {
		t.Fatalf("Invalid number of runners %v", len(np.Runners("foo")))
	}

	np.RemoveRunner("127.0.0.1:8081")
	if len(np.Runners("foo")) != len(addrs)-1 {
		t.Fatalf("Invalid number of runners %v", len(np.Runners("foo")))
	}
}
