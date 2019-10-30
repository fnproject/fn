package runnerpool

import (
	"context"
	"io"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fnproject/fn/api/models"
)

// implements RunnerCall
type dummyCall struct {
	models.Call
}

func (o *dummyCall) SlotHashId() string                     { return "" }
func (o *dummyCall) Extensions() map[string]string          { return nil }
func (o *dummyCall) RequestBody() io.ReadCloser             { return nil }
func (o *dummyCall) ResponseWriter() http.ResponseWriter    { return nil }
func (o *dummyCall) StdErr() io.ReadWriteCloser             { return nil }
func (o *dummyCall) Model() *models.Call                    { return &o.Call }
func (o *dummyCall) AddUserExecutionTime(dur time.Duration) {}
func (o *dummyCall) GetUserExecutionTime() *time.Duration   { return nil }

var _ RunnerCall = &dummyCall{}

// implements RunnerPool
type dummyPool struct {
	mock.Mock
}

func (o *dummyPool) Shutdown(ctx context.Context) error { return o.Called(ctx).Error(0) }
func (o *dummyPool) Runners(ctx context.Context, call RunnerCall) ([]Runner, error) {
	args := o.Called(ctx, call)
	return args.Get(0).([]Runner), args.Error(1)
}

var _ RunnerPool = &dummyPool{}

// implements Runner
type dummyRunner struct {
	mock.Mock
}

func (o *dummyRunner) Status(ctx context.Context) (*RunnerStatus, error) { return nil, nil }
func (o *dummyRunner) Close(ctx context.Context) error                   { return nil }
func (o *dummyRunner) Address() string                                   { return "" }
func (o *dummyRunner) TryExec(ctx context.Context, call RunnerCall) (bool, error) {
	args := o.Called(ctx, call)
	return args.Bool(0), args.Error(1)
}

var _ Runner = &dummyRunner{}

// Convenience function for us to get number of calls
func CallCount(o *mock.Mock, funcName string) int {
	var actualCalls int
	for _, call := range o.Calls {
		if call.Method == funcName {
			actualCalls++
		}
	}
	return actualCalls
}

// Simple empty list with user error, should fail quick with user error
func TestNaivePlacer_EmptyList_UserError(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
	defer cancel()

	cfg := NewPlacerConfig()
	placer := NewNaivePlacer(&cfg)

	pool := &dummyPool{}
	call := &dummyCall{}

	// api error with 502, should short cut
	poolErr := models.ErrFunctionFailed

	pool.On("Runners", ctx, call).Return([]Runner{}, poolErr)

	assert.Equal(t, poolErr, placer.PlaceCall(ctx, pool, call))
	assert.Nil(t, ctx.Err()) // no ctx timeout

	pCount := CallCount(&pool.Mock, "Runners")
	assert.True(t, pCount <= 1, "should not be spinning, hit count %d", pCount)
}

// Simple empty list without no error, should spin and wait for runners until placer timeout
func TestNaivePlacer_EmptyList_NoError(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
	defer cancel()

	cfg := NewPlacerConfig()
	cfg.PlacerTimeout = time.Duration(500 * time.Millisecond)

	placer := NewNaivePlacer(&cfg)

	pool := &dummyPool{}
	call := &dummyCall{}

	pool.On("Runners", ctx, call).Return([]Runner{}, nil)

	// we should get 503
	assert.Equal(t, models.ErrCallTimeoutServerBusy, placer.PlaceCall(ctx, pool, call))
	assert.Nil(t, ctx.Err()) // no ctx timeout (placer timeout handled internally)

	pCount := CallCount(&pool.Mock, "Runners")
	assert.True(t, pCount > 1, "should be spinning, hit count %d", pCount)
}

// Simple empty list with service error, should spin and wait for runners until placer timeout
func TestNaivePlacer_EmptyList_WithServiceError(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
	defer cancel()

	cfg := NewPlacerConfig()
	cfg.PlacerTimeout = time.Duration(500 * time.Millisecond)

	placer := NewNaivePlacer(&cfg)

	pool := &dummyPool{}
	call := &dummyCall{}

	// service error with 500, should not short cut, but should return this error
	poolErr := models.ErrCallHandlerNotFound

	pool.On("Runners", ctx, call).Return([]Runner{}, poolErr)

	// we should get service error back
	assert.Equal(t, poolErr, placer.PlaceCall(ctx, pool, call))
	assert.Nil(t, ctx.Err()) // no ctx timeout (placer timeout handled internally)

	pCount := CallCount(&pool.Mock, "Runners")
	assert.True(t, pCount > 1, "should be spinning, hit count %d", pCount)
}

// Start with simple list with no error with runners-503 twice, then empty list with user error.
// should round robin the runners twice, but then fail quick with user error once the runner
// list becomes empty.
func TestNaivePlacer_SimpleList_UserError(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
	defer cancel()

	cfg := NewPlacerConfig()
	cfg.PlacerTimeout = time.Duration(500 * time.Millisecond)
	placer := NewNaivePlacer(&cfg)

	pool := &dummyPool{}
	call := &dummyCall{}

	// two runners
	runner1 := &dummyRunner{}
	runner2 := &dummyRunner{}

	// backpressure from runners (returns "not placed" along with 503)
	runner1.On("TryExec", mock.AnythingOfType("*context.cancelCtx"), call).Return(false, models.ErrCallTimeoutServerBusy)
	runner2.On("TryExec", mock.AnythingOfType("*context.cancelCtx"), call).Return(false, models.ErrCallTimeoutServerBusy)

	// return 2-runner list twice.
	pool.On("Runners", ctx, call).Return([]Runner{runner1, runner2}, nil).Twice()

	// finally, return empty list, with api error with 502, should short cut
	poolErr := models.ErrFunctionFailed
	pool.On("Runners", ctx, call).Return([]Runner{}, poolErr).Once()

	assert.Equal(t, poolErr, placer.PlaceCall(ctx, pool, call))
	assert.Nil(t, ctx.Err()) // no ctx timeout

	pCount := CallCount(&pool.Mock, "Runners")
	assert.True(t, pCount == 3, "should not be spinning, hit count %d", pCount)

	r1Count := CallCount(&runner1.Mock, "TryExec")
	r2Count := CallCount(&runner2.Mock, "TryExec")

	assert.True(t, r1Count == 2, "runner1 round robin twice failed, hit count %d", r1Count)
	assert.True(t, r2Count == 2, "runner2 round robin twice failed, hit count %d", r2Count)
}

// Start with simple list with no error with runners-503 indefinitely
func TestNaivePlacer_SimpleList_NoError(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(2*time.Second))
	defer cancel()

	cfg := NewPlacerConfig()
	cfg.PlacerTimeout = time.Duration(500 * time.Millisecond)
	placer := NewNaivePlacer(&cfg)

	pool := &dummyPool{}
	call := &dummyCall{}

	// two runners
	runner1 := &dummyRunner{}
	runner2 := &dummyRunner{}

	// backpressure from runners (returns "not placed" along with 503)
	runner1.On("TryExec", mock.AnythingOfType("*context.cancelCtx"), call).Return(false, models.ErrCallTimeoutServerBusy)
	runner2.On("TryExec", mock.AnythingOfType("*context.cancelCtx"), call).Return(false, models.ErrCallTimeoutServerBusy)

	// return 2-runner list twice.
	pool.On("Runners", ctx, call).Return([]Runner{runner1, runner2}, nil)

	assert.Equal(t, models.ErrCallTimeoutServerBusy, placer.PlaceCall(ctx, pool, call))
	assert.Nil(t, ctx.Err()) // no ctx timeout

	pCount := CallCount(&pool.Mock, "Runners")
	assert.True(t, pCount > 1, "should be spinning hit count %d", pCount)

	r1Count := CallCount(&runner1.Mock, "TryExec")
	r2Count := CallCount(&runner2.Mock, "TryExec")

	assert.True(t, r1Count > 1, "runner1 round robin failed, not enough hit count %d", r1Count)
	assert.True(t, r2Count > 1, "runner2 round robin failed, not enough hit count %d", r2Count)

	// once the timeout occurs even in the middle of runner list processing, the hit delta
	// should not be more than 1
	assert.True(t, math.Abs(float64(r1Count)-float64(r2Count)) <= float64(1), "runner hit count inbalance")
}
