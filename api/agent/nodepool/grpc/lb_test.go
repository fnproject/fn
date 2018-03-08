package grpc

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/poolmanager"
	model "github.com/fnproject/fn/poolmanager/grpc"
)

type mockRunner struct {
	wg       sync.WaitGroup
	sleep    time.Duration
	mtx      sync.Mutex
	maxCalls int32 // Max concurrent calls
	curCalls int32 // Current calls
	addr     string
}

type mockNodePoolManager struct {
	runners []string
}

type mockgRPCNodePool struct {
	npm       poolmanager.NodePoolManager
	lbg       map[string]*lbg
	generator secureRunnerFactory
	pki       *pkiData
}

func newMockgRPCNodePool(rf secureRunnerFactory, runners []string) *mockgRPCNodePool {
	npm := &mockNodePoolManager{runners: runners}

	return &mockgRPCNodePool{
		npm:       npm,
		lbg:       make(map[string]*lbg),
		generator: rf,
		pki:       &pkiData{},
	}
}

func (npm *mockNodePoolManager) AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error {
	return nil
}

func (npm *mockNodePoolManager) GetRunners(lbgID string) ([]string, error) {
	return npm.runners, nil
}

func (npm *mockNodePoolManager) Shutdown() error {

	return nil
}

func NewMockRunnerFactory(sleep time.Duration, maxCalls int32) secureRunnerFactory {
	return func(addr string, cert string, key string, ca string) (agent.Runner, error) {
		return &mockRunner{
			sleep:    sleep,
			maxCalls: maxCalls,
			addr:     addr,
		}, nil
	}
}

func FaultyRunnerFactory() secureRunnerFactory {
	return func(addr string, cert string, key string, ca string) (agent.Runner, error) {
		return &mockRunner{
			addr: addr,
		}, errors.New("Creation of new runner failed")
	}
}

func (r *mockRunner) checkAndIncrCalls() error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if r.curCalls >= r.maxCalls {
		return models.ErrCallTimeoutServerBusy //TODO is that the correct error?
	}
	r.curCalls++
	return nil
}

func (r *mockRunner) decrCalls() {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.curCalls--
}

func (r *mockRunner) TryExec(ctx context.Context, call agent.Call) (bool, error) {
	err := r.checkAndIncrCalls()
	if err != nil {
		return false, err
	}
	defer r.decrCalls()

	r.wg.Add(1)
	defer r.wg.Done()

	time.Sleep(r.sleep)

	w, err := agent.ResponseWriter(&call)
	if err != nil {
		return true, err
	}
	buf := []byte("OK")
	(*w).Header().Set("Content-Type", "text/plain")
	(*w).Header().Set("Content-Length", strconv.Itoa(len(buf)))
	(*w).Write(buf)

	return true, nil
}

func (r *mockRunner) Close() {
	go func() {
		r.wg.Wait()
	}()
}

func (r *mockRunner) Address() string {
	return ""
}

func setupMockNodePool(lbgID string, expectedRunners []string) (*mockgRPCNodePool, *lbg) {
	rf := NewMockRunnerFactory(1*time.Millisecond, 1)
	lb := newLBG(lbgID, rf)

	np := newMockgRPCNodePool(rf, expectedRunners)
	np.lbg[lbgID] = lb
	return np, lb
}

func checkRunners(t *testing.T, expectedRunners []string, actualRunners map[string]agent.Runner, ordList []agent.Runner) {
	if len(expectedRunners) != len(actualRunners) {
		t.Errorf("List of runners is wrong, expected: %d got: %d", len(expectedRunners), len(actualRunners))
	}
	for i, r := range expectedRunners {
		_, ok := actualRunners[r]
		if !ok {
			t.Errorf("Advertised runner %s not found in the list of runners", r)
		}
		ordR := ordList[i].(*mockRunner)
		if ordR.addr != r {
			t.Error("Ordered list is not in sync with the advertised list")
		}
	}
}

func TestReloadMembersNoRunners(t *testing.T) {
	lbgID := "lb-test"
	// // Empty list, no runners available
	np, lb := setupMockNodePool(lbgID, make([]string, 0))

	np.lbg[lbgID].reloadMembers(lbgID, np.npm, np.pki)
	expectedRunners := []string{}
	ordList := lb.r_list.Load().([]agent.Runner)
	checkRunners(t, expectedRunners, lb.runners, ordList)
}

func TestReloadMembersNewRunners(t *testing.T) {
	lbgID := "lb-test"
	expectedRunners := []string{"171.16.0.1", "171.16.0.2"}
	np, lb := setupMockNodePool(lbgID, expectedRunners)

	np.lbg[lbgID].reloadMembers(lbgID, np.npm, np.pki)
	ordList := lb.r_list.Load().([]agent.Runner)
	checkRunners(t, expectedRunners, lb.runners, ordList)
}

func TestReloadMembersRemoveRunners(t *testing.T) {
	lbgID := "lb-test"
	expectedRunners := []string{"171.16.0.1", "171.16.0.3"}
	np, lb := setupMockNodePool(lbgID, expectedRunners)

	// actual runners before the update
	actualRunners := []string{"171.16.0.1", "171.16.0.2", "171.16.0.19"}
	for _, v := range actualRunners {
		r, err := lb.generator(v, np.pki.cert, np.pki.key, np.pki.ca)
		if err != nil {
			t.Error("Failed to create new runner")
		}
		lb.runners[v] = r
	}

	if len(lb.runners) != len(actualRunners) {
		t.Errorf("Failed to load list of runners")
	}

	np.lbg[lbgID].reloadMembers(lbgID, np.npm, np.pki)
	ordList := lb.r_list.Load().([]agent.Runner)
	checkRunners(t, expectedRunners, lb.runners, ordList)
}

func TestReloadMembersFailToCreateNewRunners(t *testing.T) {
	lbgID := "lb-test"
	rf := FaultyRunnerFactory()
	lb := newLBG(lbgID, rf)
	np := newMockgRPCNodePool(rf, []string{"171.19.0.1"})
	np.lbg[lbgID] = lb
	np.lbg[lbgID].reloadMembers(lbgID, np.npm, np.pki)
	actualRunners := lb.runners
	if len(actualRunners) != 0 {
		t.Errorf("List of runners should be empty")
	}
	ordList := lb.r_list.Load().([]agent.Runner)
	if ordList[0] != nullRunnerSingleton {
		t.Errorf("Ordered list should have a nullRunner")
	}
}
