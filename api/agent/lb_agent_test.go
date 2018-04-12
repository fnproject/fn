package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
)

type mockRunner struct {
	wg        sync.WaitGroup
	sleep     time.Duration
	mtx       sync.Mutex
	maxCalls  int32 // Max concurrent calls
	curCalls  int32 // Current calls
	procCalls int32 // Processed calls
	addr      string
}

type mockRunnerPool struct {
	runners   []pool.Runner
	generator pool.MTLSRunnerFactory
	pki       *pool.PKIData
}

func newMockRunnerPool(rf pool.MTLSRunnerFactory, runnerAddrs []string) *mockRunnerPool {
	var runners []pool.Runner
	for _, addr := range runnerAddrs {
		r, err := rf(addr, "", nil)
		if err != nil {
			continue
		}
		runners = append(runners, r)
	}

	return &mockRunnerPool{
		runners:   runners,
		generator: rf,
		pki:       &pool.PKIData{},
	}
}

func (rp *mockRunnerPool) Runners(call pool.RunnerCall) ([]pool.Runner, error) {
	return rp.runners, nil
}

func (rp *mockRunnerPool) Shutdown() error {
	return nil
}

func NewMockRunnerFactory(sleep time.Duration, maxCalls int32) pool.MTLSRunnerFactory {
	return func(addr, cn string, pki *pool.PKIData) (pool.Runner, error) {
		return &mockRunner{
			sleep:    sleep,
			maxCalls: maxCalls,
			addr:     addr,
		}, nil
	}
}

func FaultyRunnerFactory() pool.MTLSRunnerFactory {
	return func(addr, cn string, pki *pool.PKIData) (pool.Runner, error) {
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

func (r *mockRunner) TryExec(ctx context.Context, call pool.RunnerCall) (bool, error) {
	err := r.checkAndIncrCalls()
	if err != nil {
		return false, err
	}
	defer r.decrCalls()

	r.wg.Add(1)
	defer r.wg.Done()

	time.Sleep(r.sleep)

	r.procCalls++
	return true, nil
}

func (r *mockRunner) Close() error {
	go func() {
		r.wg.Wait()
	}()
	return nil
}

func (r *mockRunner) Address() string {
	return r.addr
}

type mockRunnerCall struct {
	lbDeadline time.Time
	r          *http.Request
	rw         http.ResponseWriter
	stdErr     io.ReadWriteCloser
	model      *models.Call
}

func (c *mockRunnerCall) LbDeadline() time.Time {
	return c.lbDeadline
}

func (c *mockRunnerCall) RequestBody() io.ReadCloser {
	return c.r.Body
}

func (c *mockRunnerCall) ResponseWriter() http.ResponseWriter {
	return c.rw
}

func (c *mockRunnerCall) StdErr() io.ReadWriteCloser {
	return c.stdErr
}

func (c *mockRunnerCall) Model() *models.Call {
	return c.model
}

func setupMockRunnerPool(expectedRunners []string, execSleep time.Duration, maxCalls int32) *mockRunnerPool {
	rf := NewMockRunnerFactory(execSleep, maxCalls)
	return newMockRunnerPool(rf, expectedRunners)
}

func TestOneRunner(t *testing.T) {
	placer := pool.NewNaivePlacer()
	rp := setupMockRunnerPool([]string{"171.19.0.1"}, 10*time.Millisecond, 5)
	call := &mockRunnerCall{lbDeadline: time.Now().Add(1 * time.Second)}
	err := placer.PlaceCall(rp, context.Background(), call)
	if err != nil {
		t.Fatalf("Failed to place call on runner %v", err)
	}
}

func TestEnforceTimeoutFromContext(t *testing.T) {
	placer := pool.NewNaivePlacer()
	rp := setupMockRunnerPool([]string{"171.19.0.1"}, 10*time.Millisecond, 5)
	call := &mockRunnerCall{lbDeadline: time.Now().Add(1 * time.Second)}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	err := placer.PlaceCall(rp, ctx, call)
	if err == nil {
		t.Fatal("Call should have timed out")
	}
}

func TestSpilloverToSecondRunner(t *testing.T) {
	placer := pool.NewNaivePlacer()
	rp := setupMockRunnerPool([]string{"171.19.0.1", "171.19.0.2"}, 10*time.Millisecond, 2)

	parallelCalls := 3
	var wg sync.WaitGroup
	failures := make(chan error, parallelCalls)
	for i := 0; i < parallelCalls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			call := &mockRunnerCall{lbDeadline: time.Now().Add(10 * time.Millisecond)}
			err := placer.PlaceCall(rp, context.Background(), call)
			if err != nil {
				failures <- fmt.Errorf("Timed out call %d", i)
			}
		}(i)
	}

	wg.Wait()
	close(failures)

	err := <-failures
	if err != nil || rp.runners[1].(*mockRunner).procCalls != 1 {
		t.Fatal("Expected spillover to second runner")
	}
}

func TestEnforceLbTimeout(t *testing.T) {
	placer := pool.NewNaivePlacer()
	rp := setupMockRunnerPool([]string{"171.19.0.1", "171.19.0.2"}, 10*time.Millisecond, 1)

	parallelCalls := 5
	var wg sync.WaitGroup
	failures := make(chan error, parallelCalls)
	for i := 0; i < parallelCalls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			call := &mockRunnerCall{lbDeadline: time.Now().Add(10 * time.Millisecond)}
			err := placer.PlaceCall(rp, context.Background(), call)
			if err != nil {
				failures <- fmt.Errorf("Timed out call %d", i)
			}
		}(i)
	}

	wg.Wait()
	close(failures)

	err := <-failures
	if err == nil {
		t.Fatal("Expected a call failure")
	}
}
