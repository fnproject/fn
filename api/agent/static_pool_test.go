package agent

import (
	"context"
	"testing"

	pool "github.com/fnproject/fn/api/runnerpool"
)

func setupStaticPool(runners []string) pool.RunnerPool {
	return NewStaticRunnerPool(runners, nil)
}

func TestNewStaticPool(t *testing.T) {
	// TEST-NET-1 unreachable
	addrs := []string{"192.0.2.255:8080", "192.0.2.255:8081"}
	np := setupStaticPool(addrs)

	runners, err := np.Runners(context.Background(), nil)
	if err != nil {
		t.Fatalf("Failed to list runners %v", err)
	}
	if len(runners) != len(addrs) {
		t.Fatalf("Invalid number of runners %v", len(runners))
	}

	err = np.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Expected no error from shutdown %v", err)
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
