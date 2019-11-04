package agent

import (
	"context"
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
	runners []pool.Runner
}

func newMockRunnerPool(sleep time.Duration, maxCalls int32, runnerAddrs []string) *mockRunnerPool {
	var runners []pool.Runner
	for _, addr := range runnerAddrs {
		r := &mockRunner{
			sleep:    sleep,
			maxCalls: maxCalls,
			addr:     addr,
		}
		runners = append(runners, r)
	}

	return &mockRunnerPool{
		runners: runners,
	}
}

func (rp *mockRunnerPool) Runners(ctx context.Context, call pool.RunnerCall) ([]pool.Runner, error) {
	return rp.runners, nil
}

func (rp *mockRunnerPool) Shutdown(context.Context) error {
	return nil
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

func (r *mockRunner) Status(ctx context.Context) (*pool.RunnerStatus, error) {
	return nil, nil
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

func (r *mockRunner) Close(context.Context) error {
	go func() {
		r.wg.Wait()
	}()
	return nil
}

func (r *mockRunner) Address() string {
	return r.addr
}

type mockRunnerCall struct {
	r          *http.Request
	rw         http.ResponseWriter
	stdErr     io.ReadWriteCloser
	model      *models.Call
	slotHashId string

	// amount of time user execution inside container
	userExecTime *time.Duration
}

func (c *mockRunnerCall) SlotHashId() string {
	return c.slotHashId
}

func (c *mockRunnerCall) Extensions() map[string]string {
	return nil
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

func (c *mockRunnerCall) AddUserExecutionTime(dur time.Duration) {
	if c.userExecTime == nil {
		c.userExecTime = new(time.Duration)
	}
	*c.userExecTime += dur
	c.Model().ExecutionDuration = *c.userExecTime
}

func (c *mockRunnerCall) GetUserExecutionTime() *time.Duration {
	return c.userExecTime
}

func setupMockRunnerPool(expectedRunners []string, execSleep time.Duration, maxCalls int32) *mockRunnerPool {
	return newMockRunnerPool(execSleep, maxCalls, expectedRunners)
}

func TestOneRunner(t *testing.T) {
	cfg := pool.NewPlacerConfig()
	placer := pool.NewNaivePlacer(&cfg)
	// TEST-NET-1 unreachable
	rp := setupMockRunnerPool([]string{"192.0.2.0"}, 10*time.Millisecond, 5)
	modelCall := &models.Call{Type: models.TypeSync}
	call := &mockRunnerCall{model: modelCall}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))
	defer cancel()
	err := placer.PlaceCall(ctx, rp, call)
	if err != nil {
		t.Fatalf("Failed to place call on runner %v", err)
	}
}

func TestEnforceTimeoutFromContext(t *testing.T) {
	cfg := pool.NewPlacerConfig()
	placer := pool.NewNaivePlacer(&cfg)
	// TEST-NET-1 unreachable
	rp := setupMockRunnerPool([]string{"192.0.2.0"}, 10*time.Millisecond, 5)

	modelCall := &models.Call{Type: models.TypeSync}
	call := &mockRunnerCall{model: modelCall}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	err := placer.PlaceCall(ctx, rp, call)
	if err == nil {
		t.Fatal("Call should have timed out")
	}
}

func TestDetachedPlacerTimeout(t *testing.T) {
	// In this test we set the detached placer timeout to a value lower than the request timeout (call.Timeout)
	// the fake placer will just sleep for a time greater of the detached placement timeout and it will return
	// the right error only if the detached timeout exceeds but the request timeout is still valid
	cfg := pool.NewPlacerConfig()
	cfg.DetachedPlacerTimeout = 300 * time.Millisecond
	placer := pool.NewFakeDetachedPlacer(&cfg, 400*time.Millisecond)

	modelCall := &models.Call{Type: models.TypeDetached}
	call := &mockRunnerCall{model: modelCall}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(30*time.Second))
	defer cancel()
	err := placer.PlaceCall(ctx, nil, call)
	if err == nil {
		t.Fatal("Detached call should have time out because of the expiration of the placement timeout")
	}

}

func TestEnforceLbTimeout(t *testing.T) {
	cfg := pool.NewPlacerConfig()
	placer := pool.NewNaivePlacer(&cfg)
	// TEST-NET-1 unreachable
	rp := setupMockRunnerPool([]string{"192.0.2.0", "192.0.2.1"}, 10*time.Millisecond, 1)

	parallelCalls := 5
	var wg sync.WaitGroup
	failures := make(chan error, parallelCalls)
	for i := 0; i < parallelCalls; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Millisecond))
			defer cancel()

			modelCall := &models.Call{Type: models.TypeSync}
			call := &mockRunnerCall{model: modelCall}

			err := placer.PlaceCall(ctx, rp, call)
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

// SetCallType create a models.Call setting up the provided Call Type
func SetCallType(callType string) CallOpt {
	return func(c *call) error {
		c.Call = &models.Call{Type: callType}
		c.req, _ = http.NewRequest("GET", "http://www.example.com", nil)
		return nil
	}
}

func ModifyCallRequest(callType string) CallOpt {
	return func(c *call) error {
		c.Call.Type = callType
		return nil
	}
}
func TestGetCallSetOpts(t *testing.T) {
	expected := models.TypeSync
	a, err := NewLBAgent(nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error in creating LB Agent, %s", err.Error())
	}
	c, err := a.GetCall(SetCallType(expected))
	if err != nil {
		t.Fatalf("Unexpected error calling GetCall, %s", err.Error())
	}

	actual := c.Model().Type
	if expected != actual {
		t.Fatalf("Expected %s got %s", expected, actual)
	}
}

// We verify that we can add callOptions which are executed after the option supplied to  GetCall
func TestWithLBCallOptions(t *testing.T) {
	expected := models.TypeDetached
	a, err := NewLBAgent(nil, nil, WithLBCallOptions(ModifyCallRequest(expected)))
	if err != nil {
		t.Fatalf("Unexpected error in creating LB Agent, %s", err.Error())
	}

	lbAgent, ok := a.(*lbAgent)
	if !ok {
		t.Fatal("NewLBAgent doesn't return an lbAgent")
	}

	c, err := lbAgent.GetCall(SetCallType(models.TypeSync))
	if err != nil {
		t.Fatalf("Unexpected error calling GetCall, %s", err.Error())
	}

	actual := len(lbAgent.callOpts)
	if actual != 1 {
		t.Fatalf("Expected 1 call Options got %d call Option", actual)
	}

	actualType := c.Model().Type
	if expected != actualType {
		t.Fatalf("Expected %s got %s", expected, actualType)
	}
}
