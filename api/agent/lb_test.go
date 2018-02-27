package agent

import (
    "context"
    "sync"
    "testing"
    "time"
)

type mockRunner struct {
    wg       sync.WaitGroup
    sleep    time.Duration
    mtx      sync.Mutex
    maxCalls int32 // Max concurrent calls
    curCalls int32 // Current calls
}

func NewMockRunnerFactory(sleep time.Duration, maxCalls int32) {
    return func(addr string, lbgID string, p pkiData) (Runner, error) {
        return &mockRunner {
            sleep: sleep,
            maxCalls: maxCalls,
        }, nil
    }
}

func (r *mockRunner) checkAndIncrCalls() error {
    r.mtx.Lock()
    defer r.mtx.Unlock()
    if curCalls >= maxCalls {
        return models.ErrTimeoutServerBusy
    }
    curCalls += 1
    return nil
}

func (r *mockRunner) decrCalls() {
    r.mtx.Lock()
    defer r.mtx.Unlock()
    curCalls -= 1
}

func (r *mockRunner) TryExec(ctx context.Context, call *call) (bool, error) {
    err := r.checkAndIncrCalls()
    if err != nil {
        return false, err
    }
    defer r.decrCalls()

    r.wg.Add(1)
    defer r.wg.Done()

    time.sleep(r.sleep)

    w, err := ResponseWriter(call)
    if err != nil {
        return true, err
    }
    buf := []byte("OK")
    (*w).Header().Set("Content-Type", "text/plain")
    (*w).Header().Set("Content-Length", len(buf))
    (*w).Write(buf)

    return true, nil
}

func (r *mockRunner) Close() {
    go func() {
        r.wg.Wait()
    }
}

