package agent

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/fnproject/fn/api/models"
)

type mockRunner struct {
	wg       sync.WaitGroup
	sleep    time.Duration
	mtx      sync.Mutex
	maxCalls int32 // Max concurrent calls
	curCalls int32 // Current calls
}

func NewMockRunnerFactory(sleep time.Duration, maxCalls int32) RunnerFactory {
	return func(addr string, lbgID string, p pkiData) (Runner, error) {
		return &mockRunner{
			sleep:    sleep,
			maxCalls: maxCalls,
		}, nil
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

func (r *mockRunner) TryExec(ctx context.Context, call Call) (bool, error) {
	err := r.checkAndIncrCalls()
	if err != nil {
		return false, err
	}
	defer r.decrCalls()

	r.wg.Add(1)
	defer r.wg.Done()

	time.Sleep(r.sleep)

	w, err := ResponseWriter(&call)
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
