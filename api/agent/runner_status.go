package agent

import (
	"bytes"
	"context"
	"encoding/json"
	runner "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/empty"
	pbst "github.com/golang/protobuf/ptypes/struct"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const v1StatusRequest = "{}"

// statusTracker maintains cache data/state/locks for Status Call invocations.
type statusTracker struct {
	agent                   Agent //  Agent used to run the status image (call)
	customHealthCheckerFunc func(context.Context) (map[string]string, error)

	inflight         int32
	requestsReceived uint64
	requestsHandled  uint64
	kdumpsOnDisk     uint64
	imageName        string

	// if file exists, then network in status checks is enabled.
	barrierPath string

	// lock protects expiry/cache/wait fields below. RunnerStatus ptr itself
	// stored every time status image is executed. Cache fetches use a shallow
	// copy of RunnerStatus to ensure consistency. Shallow copy is sufficient
	// since we set/save contents of RunnerStatus once.
	lock   sync.Mutex
	expiry time.Time
	cache  *runner.RunnerStatus
	wait   chan struct{}
}

func NewStatusTracker() *statusTracker {
	return &statusTracker{}
}

func NewStatusTrackerWithAgent(a Agent) *statusTracker {
	st := &statusTracker{}
	st.agent = a
	return st
}

func (st *statusTracker) setAgent(a Agent) {
	st.agent = a
}

func (st *statusTracker) Status(ctx context.Context, _ *empty.Empty) (*runner.RunnerStatus, error) {
	return st.statusV2(ctx, json.RawMessage(v1StatusRequest))
}

func (st *statusTracker) Status2(ctx context.Context, r *pbst.Struct) (*runner.RunnerStatus, error) {

	b := bytes.Buffer{}
	m := &jsonpb.Marshaler{}
	err := m.Marshal(&b, r)
	if err != nil {
		common.Logger(ctx).WithError(err).Warnf("status call: failed to marshal request %v", err)
		return nil, err
	}
	return st.statusV2(ctx, b.Bytes())
}

func (st *statusTracker) statusV2(ctx context.Context, req json.RawMessage) (*runner.RunnerStatus, error) {
	// Status using image name is disabled. We return inflight request count only
	if st.imageName == "" {
		return &runner.RunnerStatus{
			Active:           atomic.LoadInt32(&st.inflight),
			RequestsReceived: atomic.LoadUint64(&st.requestsReceived),
			RequestsHandled:  atomic.LoadUint64(&st.requestsHandled),
		}, nil
	}
	status, err := st.handleStatusCall(ctx, req)
	if err != nil && err != context.Canceled {
		common.Logger(ctx).WithError(err).Warnf("Status call failed result=%+v", status)
	}

	cached := "error"
	success := "error"
	network := "error"

	if err == nil && status != nil {
		cached = strconv.FormatBool(status.Cached)
		success = strconv.FormatBool(!status.Failed)
		network = strconv.FormatBool(!status.IsNetworkDisabled)
	}
	statsStatusCall(ctx, cached, success, network)

	return status, err
}

// Handles a status call concurrency and caching.
func (st *statusTracker) handleStatusCall(ctx context.Context, req json.RawMessage) (*runner.RunnerStatus, error) {

	waitChan, isSpawner := st.checkStatusCall(ctx)

	// from cache
	if waitChan == nil {
		return st.fetchStatusCall(ctx)
	}

	if isSpawner {
		st.spawnStatusCall(ctx, req)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-waitChan:
		return st.fetchStatusCall(ctx)
	}
}

// Runs a status call using status image with baked in parameters.
func (st *statusTracker) runStatusCall(ctx context.Context, req json.RawMessage) *runner.RunnerStatus {
	// IMPORTANT: apply an upper bound timeout
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, StatusCtxTimeout)
	defer cancel()

	result := &runner.RunnerStatus{}
	log := common.Logger(ctx)

	// construct call

	c := st.newStatusCall(req)
	// TODO: reliably shutdown this container after executing one request.

	log.Debugf("Running status call with id=%v image=%v", c.ID, c.Image)

	recorder := httptest.NewRecorder()
	player := ioutil.NopCloser(strings.NewReader(c.Payload))

	// Fetch network status
	if st.barrierPath != "" {
		_, err := os.Lstat(st.barrierPath)
		if err != nil {
			result.IsNetworkDisabled = true
		}
	}

	var err error
	// handle custom healthcheck
	if st.customHealthCheckerFunc != nil {
		result.CustomStatus, err = st.customHealthCheckerFunc(ctx)
	}

	// TODO: Raise en error if don't have an agent
	//       Possible if constructed without agent and
	//       callers forgot to set one
	var agentCall Call
	var mcall *call
	if err == nil {
		agentCall, err = st.agent.GetCall(FromModelAndInput(&c, player),
			WithLogger(common.NoopReadWriteCloser{}),
			WithWriter(recorder),
			WithContext(ctx),
		)
	}

	if err == nil {
		mcall = agentCall.(*call)

		// disable network if not ready
		mcall.disableNet = result.IsNetworkDisabled

		err = st.agent.Submit(mcall)
	}

	resp := recorder.Result()

	if err != nil {
		result.ErrorCode = int32(models.GetAPIErrorCode(err))
		result.ErrorStr = err.Error()
		result.Failed = true
	} else if resp.StatusCode >= http.StatusBadRequest {
		result.ErrorCode = int32(resp.StatusCode)
		result.Failed = true
	}

	schedDur, execDur := GetCallLatencies(mcall)
	result.SchedulerDuration = int64(schedDur)
	result.ExecutionDuration = int64(execDur)

	// These timestamps are related. To avoid confusion
	// and for robustness, nested if stmts below.
	if !time.Time(c.CreatedAt).IsZero() {
		result.CreatedAt = c.CreatedAt.String()

		if !time.Time(c.StartedAt).IsZero() {
			result.StartedAt = c.StartedAt.String()

			if !time.Time(c.CompletedAt).IsZero() {
				result.CompletedAt = c.CompletedAt.String()
			} else {
				// IMPORTANT: We punch this in ourselves.
				result.CompletedAt = common.DateTime(time.Now()).String()
			}
		}
	}

	// Loading with runHot metrics if not nil
	if mcall != nil {
		result.ImagePullWaitDuration = atomic.LoadInt64(&mcall.imagePullWaitTime)
		result.CtrCreateDuration = atomic.LoadInt64(&mcall.ctrCreateTime)
		result.CtrPrepDuration = atomic.LoadInt64(&mcall.ctrPrepTime)
		result.InitStartTime = atomic.LoadInt64(&mcall.initStartTime)
	}

	// Status images should not output excessive data since we echo the
	// data back to caller.
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	result.Details = string(body)
	result.Id = c.ID

	log.Debugf("Finished status call id=%v result=%+v", c.ID, result)
	return result
}

func (st *statusTracker) spawnStatusCall(ctx context.Context, payload json.RawMessage) {
	go func() {
		var waitChan chan struct{}
		// IMPORTANT: We have to strip client timeouts to make sure this completes
		// in the background even if client cancels/times out.
		cachePtr := st.runStatusCall(common.BackgroundContext(ctx), payload)
		now := time.Now()

		// Pointer store of 'cachePtr' is sufficient here as isWaiter/isCached above perform a shallow
		// copy of 'cache'
		st.lock.Lock()

		st.cache = cachePtr
		st.expiry = now.Add(StatusCallCacheDuration)
		waitChan = st.wait // cannot be null
		st.wait = nil

		st.lock.Unlock()

		// signal waiters
		close(waitChan)
	}()
}

func (st *statusTracker) fetchStatusCall(ctx context.Context) (*runner.RunnerStatus, error) {
	var cacheObj runner.RunnerStatus

	// A shallow copy is sufficient here, as we do not modify nested data in
	// RunnerStatus in any way.
	st.lock.Lock()

	cacheObj = *st.cache // cannot be null
	// deepcopy of custom healthcheck status is needed
	cacheObj.CustomStatus = make(map[string]string)
	for k, v := range st.cache.CustomStatus {
		cacheObj.CustomStatus[k] = v
	}

	st.cache.Cached = true

	st.lock.Unlock()

	// The rest of the RunnerStatus fields are not cached and always populated
	// with latest metrics.
	cacheObj.Active = atomic.LoadInt32(&st.inflight)
	cacheObj.RequestsReceived = atomic.LoadUint64(&st.requestsReceived)
	cacheObj.RequestsHandled = atomic.LoadUint64(&st.requestsHandled)
	cacheObj.KdumpsOnDisk = atomic.LoadUint64(&st.kdumpsOnDisk)

	return &cacheObj, ctx.Err()
}

func (st *statusTracker) checkStatusCall(ctx context.Context) (chan struct{}, bool) {
	now := time.Now()

	st.lock.Lock()
	defer st.lock.Unlock()

	// cached?
	if st.expiry.After(now) {
		return nil, false
	}

	// already running?
	if st.wait != nil {
		return st.wait, false
	}

	// spawn a new call
	st.wait = make(chan struct{})
	return st.wait, true
}

func (st *statusTracker) newStatusCall(payload json.RawMessage) models.Call {

	var c models.Call

	// Most of these arguments are baked in. We might want to make this
	// more configurable.
	c.ID = id.New().String()
	c.Image = st.imageName
	c.Type = models.TypeSync
	c.TmpFsSize = 0
	// IMPORTANT: mem/cpu set to zero. This means status containers cannot be evicted.
	c.Memory = 0
	c.CPUs = models.MilliCPUs(0)
	c.URL = "/"
	c.Method = "GET"
	c.CreatedAt = common.DateTime(time.Now())
	c.Config = make(models.Config)
	c.Payload = string(payload)
	c.Timeout = StatusCallTimeout
	c.IdleTimeout = StatusCallIdleTimeout

	return c
}
