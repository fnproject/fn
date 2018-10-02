package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/fnproject/fn/grpcutil"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

/*
	Pure Runner (implements Agent) proxies gRPC requests to the actual Agent instance. This is
	done using http.ResponseWriter interfaces where Agent pushes the function I/O through:
	1) Function output to pure runner is received through callHandle http.ResponseWriter interface.
	2) Function input from pure runner to Agent is processed through callHandle io.PipeWriter.
	3) LB to runner input is handled via receiver (inQueue)
	4) runner to LB output is handled via sender (outQueue)

	The flow of events is as follows:

	1) LB sends ClientMsg_Try to runner
	2) Runner allocates its resources and sends an ACK: RunnerMsg_Acknowledged
	3) LB sends ClientMsg_Data messages with an EOF for last message set.
	4) Runner upon receiving with ClientMsg_Data calls agent.Submit()
	5) agent.Submit starts reading data from callHandle io.PipeReader, this reads
		data from LB via gRPC receiver (inQueue).
	6) agent.Submit starts sending data via callHandle http.ResponseWriter interface,
		which is pushed to gRPC sender (outQueue) to the LB.
	7) agent.Submit() completes, this means, the Function I/O is now completed.
	8) Runner finalizes gRPC session with RunnerMsg_Finished to LB.

*/

var (
	ErrorExpectedTry  = errors.New("Protocol failure: expected ClientMsg_Try")
	ErrorExpectedData = errors.New("Protocol failure: expected ClientMsg_Data")
)

// callHandle represents the state of the call as handled by the pure runner, and additionally it implements the
// interface of http.ResponseWriter so that it can be used for streaming the output back.
type callHandle struct {
	engagement runner.RunnerProtocol_EngageServer
	ctx        context.Context
	c          *call // the agent's version of call

	// For implementing http.ResponseWriter:
	headers http.Header
	status  int

	headerOnce        sync.Once
	shutOnce          sync.Once
	pipeToFnCloseOnce sync.Once

	outQueue  chan *runner.RunnerMsg
	doneQueue chan struct{}
	errQueue  chan error
	inQueue   chan *runner.ClientMsg

	// Pipe to push data to the agent Function container
	pipeToFnW *io.PipeWriter
	pipeToFnR *io.PipeReader
}

func NewCallHandle(engagement runner.RunnerProtocol_EngageServer) *callHandle {

	// set up a pipe to push data to agent Function container
	pipeR, pipeW := io.Pipe()

	state := &callHandle{
		engagement: engagement,
		ctx:        engagement.Context(),
		headers:    make(http.Header),
		status:     200,
		outQueue:   make(chan *runner.RunnerMsg),
		doneQueue:  make(chan struct{}),
		errQueue:   make(chan error, 1), // always allow one error (buffered)
		inQueue:    make(chan *runner.ClientMsg),
		pipeToFnW:  pipeW,
		pipeToFnR:  pipeR,
	}

	// spawn one receiver and one sender go-routine.
	// See: https://grpc.io/docs/reference/go/generated-code.html, which reads:
	//   "Thread-safety: note that client-side RPC invocations and server-side RPC handlers
	//   are thread-safe and are meant to be run on concurrent goroutines. But also note that
	//   for individual streams, incoming and outgoing data is bi-directional but serial;
	//   so e.g. individual streams do not support concurrent reads or concurrent writes
	//   (but reads are safely concurrent with writes)."
	state.spawnReceiver()
	state.spawnSender()
	return state
}

// closePipeToFn closes the pipe that feeds data to the function in agent.
func (ch *callHandle) closePipeToFn() {
	ch.pipeToFnCloseOnce.Do(func() {
		ch.pipeToFnW.Close()
	})
}

// finalize initiates a graceful shutdown of the session. This is
// currently achieved by a sentinel nil enqueue to gRPC sender.
func (ch *callHandle) finalize() error {
	// final sentinel nil msg for graceful shutdown
	err := ch.enqueueMsg(nil)
	if err != nil {
		ch.shutdown(err)
	}
	return err
}

// shutdown initiates a shutdown and terminates the gRPC session with
// a given error.
func (ch *callHandle) shutdown(err error) {

	ch.closePipeToFn()

	ch.shutOnce.Do(func() {
		common.Logger(ch.ctx).WithError(err).Debugf("Shutting down call handle")

		// try to queue an error message if it's not already queued.
		if err != nil {
			select {
			case ch.errQueue <- err:
			default:
			}
		}

		close(ch.doneQueue)
	})
}

// waitError waits until the session is completed and results
// any queued error if there is any.
func (ch *callHandle) waitError() error {
	select {
	case <-ch.ctx.Done():
	case <-ch.doneQueue:
	}

	var err error
	// get queued error if there's any
	select {
	case err = <-ch.errQueue:
	default:
		err = ch.ctx.Err()
	}

	if err != nil {
		logrus.WithError(err).Debugf("Wait Error")
	}
	return err
}

// enqueueMsg attempts to queue a message to the gRPC sender
func (ch *callHandle) enqueueMsg(msg *runner.RunnerMsg) error {
	select {
	case ch.outQueue <- msg:
		return nil
	case <-ch.ctx.Done():
	case <-ch.doneQueue:
	}
	return io.EOF
}

// enqueueMsgStricy enqueues a message to the gRPC sender and if
// that fails then initiates an error case shutdown.
func (ch *callHandle) enqueueMsgStrict(msg *runner.RunnerMsg) error {
	err := ch.enqueueMsg(msg)
	if err != nil {
		ch.shutdown(err)
	}
	return err
}

func (ch *callHandle) enqueAckSync(err error) {
	statusCode := http.StatusAccepted
	if err != nil {
		if models.IsAPIError(err) {
			statusCode = models.GetAPIErrorCode(err)
		} else {
			statusCode = http.StatusInternalServerError
		}
	}

	err = ch.enqueueMsg(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_ResultStart{
			ResultStart: &runner.CallResultStart{
				Meta: &runner.CallResultStart_Http{
					Http: &runner.HttpRespMeta{
						Headers:    ch.prepHeaders(),
						StatusCode: int32(statusCode)}}}}})
}

// enqueueCallResponse enqueues a Submit() response to the LB
// and initiates a graceful shutdown of the session.
func (ch *callHandle) enqueueCallResponse(err error) {

	var createdAt string
	var startedAt string
	var completedAt string
	var details string
	var errCode int
	var errStr string

	log := common.Logger(ch.ctx)

	if err != nil {
		errCode = models.GetAPIErrorCode(err)
		errStr = err.Error()
	}

	if ch.c != nil {
		mcall := ch.c.Model()

		// These timestamps are related. To avoid confusion
		// and for robustness, nested if stmts below.
		if !time.Time(mcall.CreatedAt).IsZero() {
			createdAt = mcall.CreatedAt.String()

			if !time.Time(mcall.StartedAt).IsZero() {
				startedAt = mcall.StartedAt.String()

				if !time.Time(mcall.CompletedAt).IsZero() {
					completedAt = mcall.CompletedAt.String()
				} else {
					// IMPORTANT: We punch this in ourselves.
					// This is because call.End() is executed asynchronously.
					completedAt = common.DateTime(time.Now()).String()
				}
			}
		}

		details = mcall.ID

	}
	log.Debugf("Sending Call Finish details=%v", details)

	errTmp := ch.enqueueMsgStrict(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
			Success:     err == nil,
			Details:     details,
			ErrorCode:   int32(errCode),
			ErrorStr:    errStr,
			CreatedAt:   createdAt,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
		}}})

	if errTmp != nil {
		log.WithError(errTmp).Infof("enqueueCallResponse Send Error details=%v err=%v:%v", details, errCode, errStr)
		return
	}

	errTmp = ch.finalize()
	if errTmp != nil {
		log.WithError(errTmp).Infof("enqueueCallResponse Finalize Error details=%v err=%v:%v", details, errCode, errStr)
	}
}

// spawnPipeToFn pumps data to Function via callHandle io.PipeWriter (pipeToFnW)
// which is fed using input channel.
func (ch *callHandle) spawnPipeToFn() chan *runner.DataFrame {

	input := make(chan *runner.DataFrame)
	go func() {
		defer ch.closePipeToFn()
		for {
			select {
			case <-ch.doneQueue:
				return
			case <-ch.ctx.Done():
				return
			case data := <-input:
				if data == nil {
					return
				}

				if len(data.Data) > 0 {
					_, err := io.CopyN(ch.pipeToFnW, bytes.NewReader(data.Data), int64(len(data.Data)))
					if err != nil {
						ch.shutdown(err)
						return
					}
				}
				if data.Eof {
					return
				}
			}
		}
	}()

	return input
}

// spawnReceiver starts a gRPC receiver, which
// feeds received LB messages into inQueue
func (ch *callHandle) spawnReceiver() {

	go func() {
		defer close(ch.inQueue)
		for {
			msg, err := ch.engagement.Recv()
			if err != nil {
				// engagement is close/cancelled from client.
				if err == io.EOF {
					return
				}
				ch.shutdown(err)
				return
			}

			select {
			case ch.inQueue <- msg:
			case <-ch.doneQueue:
				return
			case <-ch.ctx.Done():
				return
			}
		}
	}()
}

// spawnSender starts a gRPC sender, which
// pumps messages from outQueue to the LB.
func (ch *callHandle) spawnSender() {
	go func() {
		for {
			select {
			case msg := <-ch.outQueue:
				if msg == nil {
					ch.shutdown(nil)
					return
				}
				err := ch.engagement.Send(msg)
				if err != nil {
					ch.shutdown(err)
					return
				}
			case <-ch.doneQueue:
				return
			case <-ch.ctx.Done():
				return
			}
		}
	}()
}

// Header implements http.ResponseWriter, which
// is used by Agent to push headers to pure runner
func (ch *callHandle) Header() http.Header {
	return ch.headers
}

// WriteHeader implements http.ResponseWriter, which
// is used by Agent to push http status to pure runner
func (ch *callHandle) WriteHeader(status int) {
	ch.status = status
}

// prepHeaders is a utility function to compile http headers
// into a flat array.
func (ch *callHandle) prepHeaders() []*runner.HttpHeader {
	var headers []*runner.HttpHeader
	for h, vals := range ch.headers {
		for _, v := range vals {
			headers = append(headers, &runner.HttpHeader{
				Key:   h,
				Value: v,
			})
		}
	}
	return headers
}

// Write implements http.ResponseWriter, which
// is used by Agent to push http data to pure runner. The
// received data is pushed to LB via gRPC sender queue.
// Write also sends http headers/state to the LB.
func (ch *callHandle) Write(data []byte) (int, error) {
	if ch.c.Model().Type == models.TypeAsync {
		//If it is an acksync call we just /dev/null the data coming back from the container
		return len(data), nil
	}
	var err error
	ch.headerOnce.Do(func() {
		// WARNING: we do fetch Status and Headers without
		// a lock below. This is a problem in agent in general, and needs
		// to be fixed in all accessing go-routines such as protocol/http.go,
		// protocol/json.go, agent.go, etc. In practice however, one go routine
		// accesses them (which also compiles and writes headers), but this
		// is fragile and needs to be fortified.
		err = ch.enqueueMsg(&runner.RunnerMsg{
			Body: &runner.RunnerMsg_ResultStart{
				ResultStart: &runner.CallResultStart{
					Meta: &runner.CallResultStart_Http{
						Http: &runner.HttpRespMeta{
							Headers:    ch.prepHeaders(),
							StatusCode: int32(ch.status),
						},
					},
				},
			},
		})
	})

	if err != nil {
		return 0, err
	}
	total := 0
	// split up data into gRPC chunks
	for {
		chunkSize := len(data)
		if chunkSize > MaxDataChunk {
			chunkSize = MaxDataChunk
		}
		if chunkSize == 0 {
			break
		}

		// we cannot retain 'data'
		cpData := make([]byte, chunkSize)
		copy(cpData, data[0:chunkSize])
		data = data[chunkSize:]

		err = ch.enqueueMsg(&runner.RunnerMsg{
			Body: &runner.RunnerMsg_Data{
				Data: &runner.DataFrame{
					Data: cpData,
					Eof:  false,
				},
			},
		})

		if err != nil {
			return total, err
		}
		total += chunkSize
	}

	return total, nil
}

// getTryMsg fetches/waits for a TryCall message from
// the LB using inQueue (gRPC receiver)
func (ch *callHandle) getTryMsg() *runner.TryCall {
	var msg *runner.TryCall

	select {
	case <-ch.doneQueue:
	case <-ch.ctx.Done():
		// if ctx timed out while waiting, then this is a 503 (retriable)
		err := status.Errorf(codes.Code(models.ErrCallTimeoutServerBusy.Code()), models.ErrCallTimeoutServerBusy.Error())
		ch.shutdown(err)
		return nil
	case item := <-ch.inQueue:
		if item != nil {
			msg = item.GetTry()
		}
	}
	if msg == nil {
		ch.shutdown(ErrorExpectedTry)
	}
	return msg
}

// getDataMsg fetches/waits for a DataFrame message from
// the LB using inQueue (gRPC receiver)
func (ch *callHandle) getDataMsg() *runner.DataFrame {
	var msg *runner.DataFrame

	select {
	case <-ch.doneQueue:
	case <-ch.ctx.Done():
	case item := <-ch.inQueue:
		if item != nil {
			msg = item.GetData()
		}
	}
	if msg == nil {
		ch.shutdown(ErrorExpectedData)
	}
	return msg
}

const (
	// Here we give 5 seconds of timeout inside the container. We hardcode these numbers here to
	// ensure we control idle timeout & timeout as well as how long should cache be valid.
	// A cache duration of idleTimeout + 500 msecs allows us to reuse the cache, for about 1.5 secs,
	// and during this time, since we allow no queries to go through, the hot container times out.
	//
	// For now, status tests a single case: a new hot container is spawned when cache is expired
	// and when a query is allowed to run.
	// TODO: we might want to mix this up and perhaps allow that hot container to handle
	// more than one query to test both 'new hot container' and 'old hot container' cases.
	StatusCallTimeout       = int32(5)
	StatusCallIdleTimeout   = int32(1)
	StatusCallCacheDuration = time.Duration(500)*time.Millisecond + time.Duration(StatusCallIdleTimeout)*time.Second

	// Total context timeout (scheduler+execution.) We need to allocate plenty of time here.
	// 60 seconds should be enough to provoke disk I/O errors, docker timeouts. etc.
	StatusCtxTimeout = time.Duration(60 * time.Second)
)

// statusTracker maintains cache data/state/locks for Status Call invocations.
type statusTracker struct {
	inflight         int32
	requestsReceived uint64
	requestsHandled  uint64
	imageName        string

	// lock protects expiry/cache/wait fields below. RunnerStatus ptr itself
	// stored every time status image is executed. Cache fetches use a shallow
	// copy of RunnerStatus to ensure consistency. Shallow copy is sufficient
	// since we set/save contents of RunnerStatus once.
	lock   sync.Mutex
	expiry time.Time
	cache  *runner.RunnerStatus
	wait   chan struct{}
}

// pureRunner implements Agent and delegates execution of functions to an internal Agent; basically it wraps around it
// and provides the gRPC server that implements the LB <-> Runner protocol.
type pureRunner struct {
	gRPCServer *grpc.Server
	creds      credentials.TransportCredentials
	a          Agent
	status     statusTracker
}

// implements Agent
func (pr *pureRunner) GetCall(opts ...CallOpt) (Call, error) {
	return pr.a.GetCall(opts...)
}

// implements Agent
func (pr *pureRunner) Submit(Call) error {
	return errors.New("Submit cannot be called directly in a Pure Runner.")
}

// implements Agent
func (pr *pureRunner) Close() error {
	// First stop accepting requests
	pr.gRPCServer.GracefulStop()
	// Then let the agent finish
	err := pr.a.Close()
	if err != nil {
		return err
	}
	return nil
}

// implements Agent
func (pr *pureRunner) AddCallListener(cl fnext.CallListener) {
	pr.a.AddCallListener(cl)
}

func (pr *pureRunner) spawnSubmit(state *callHandle, chErr chan error) {
	go func() {
		err := pr.a.Submit(state.c)
		// Notify the error back
		chErr <- err
		state.enqueueCallResponse(err)
	}()
}

// handleTryCall based on the TryCall message, tries to place the call on NBIO Agent
func (pr *pureRunner) handleTryCall(tc *runner.TryCall, state *callHandle) error {

	start := time.Now()

	var c models.Call
	err := json.Unmarshal([]byte(tc.ModelsCallJson), &c)
	if err != nil {
		state.enqueueCallResponse(err)
		return err
	}

	// Status image is reserved for internal Status checks.
	// We need to make sure normal functions calls cannot call it.
	if pr.status.imageName != "" && c.Image == pr.status.imageName {
		err = models.ErrFnsInvalidImage
		state.enqueueCallResponse(err)
		return err
	}

	// IMPORTANT: We clear/initialize these dates as start/created/completed dates from
	// unmarshalled Model from LB-agent represent unrelated time-line events.
	// From this point, CreatedAt/StartedAt/CompletedAt are based on our local clock.
	c.CreatedAt = common.DateTime(start)
	c.StartedAt = common.DateTime(time.Time{})
	c.CompletedAt = common.DateTime(time.Time{})

	agent_call, err := pr.a.GetCall(FromModelAndInput(&c, state.pipeToFnR),
		WithWriter(state),
		WithContext(state.ctx),
		WithExtensions(tc.GetExtensions()),
	)
	if err != nil {
		state.enqueueCallResponse(err)
		return err
	}

	state.c = agent_call.(*call)
	if tc.SlotHashId != "" {
		hashId, err := hex.DecodeString(tc.SlotHashId)
		if err != nil {
			state.enqueueCallResponse(err)
			return err
		}
		state.c.slotHashId = string(hashId[:])
	}
	errSubmit := make(chan error, 1)
	state.c.ackSync = make(chan error, 1)
	pr.spawnSubmit(state, errSubmit)
	select {
	case <-errSubmit:
		// The error is already managed in the spawnSubmit we just return here
		return nil
	case err := <-state.c.ackSync:
		// if it is an acksync then we want to notify the LB
		if state.c.Model().Type == models.TypeAcksync {
			// EnqueuSyncAck
			state.enqueAckSync(err)
		}
	}

	return nil
}

// implements RunnerProtocolServer
// Handles a client engagement
func (pr *pureRunner) Engage(engagement runner.RunnerProtocol_EngageServer) error {
	grpc.EnableTracing = false
	ctx := engagement.Context()
	log := common.Logger(ctx)
	// Keep lightweight tabs on what this runner is doing: for draindown tests
	atomic.AddInt32(&pr.status.inflight, 1)
	atomic.AddUint64(&pr.status.requestsReceived, 1)

	pv, ok := peer.FromContext(ctx)
	log.Debug("Starting engagement")
	if ok {
		log.Debug("Peer is ", pv)
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		log.Debug("MD is ", md)
	}
	state := NewCallHandle(engagement)

	tryMsg := state.getTryMsg()
	if tryMsg != nil {
		errTry := pr.handleTryCall(tryMsg, state)
		if errTry == nil {
			dataFeed := state.spawnPipeToFn()

		DataLoop:
			for {
				dataMsg := state.getDataMsg()
				if dataMsg == nil {
					break
				}

				select {
				case dataFeed <- dataMsg:
					if dataMsg.Eof {
						break DataLoop
					}
				case <-state.doneQueue:
					break DataLoop
				case <-state.ctx.Done():
					break DataLoop
				}
			}
		}
	}

	err := state.waitError()

	// if we didn't respond with TooBusy, then this means the request
	// was processed.
	if err != models.ErrCallTimeoutServerBusy {
		atomic.AddUint64(&pr.status.requestsHandled, 1)
	}

	atomic.AddInt32(&pr.status.inflight, -1)
	return err
}

// Runs a status call using status image with baked in parameters.
func (pr *pureRunner) runStatusCall(ctx context.Context) *runner.RunnerStatus {

	// IMPORTANT: We have to use our own context with a set timeout in order
	// to ignore client timeouts. Original 'ctx' here carries client side deadlines.
	// Since these deadlines can vary, in order to make sure Status runs predictably we
	// use a hardcoded preset timeout 'ctxTimeout' instead.
	// TODO: It would be good to copy original ctx key/value pairs into our new
	// context for tracking, etc. But go-lang context today does not seem to allow this.
	execCtx, execCtxCancel := context.WithTimeout(context.Background(), StatusCtxTimeout)
	defer execCtxCancel()

	result := &runner.RunnerStatus{}
	log := common.Logger(ctx)
	start := time.Now()

	// construct call
	var c models.Call

	// Most of these arguments are baked in. We might want to make this
	// more configurable.
	c.ID = id.New().String()
	c.Image = pr.status.imageName
	c.Type = "sync"
	c.Format = "json"
	c.TmpFsSize = 0
	c.Memory = 0
	c.CPUs = models.MilliCPUs(0)
	c.URL = "/"
	c.Method = "GET"
	c.CreatedAt = common.DateTime(start)
	c.Config = make(models.Config)
	c.Config["FN_FORMAT"] = c.Format
	c.Payload = "{}"
	c.Timeout = StatusCallTimeout
	c.IdleTimeout = StatusCallIdleTimeout

	// TODO: reliably shutdown this container after executing one request.

	log.Debugf("Running status call with id=%v image=%v", c.ID, c.Image)

	recorder := httptest.NewRecorder()
	player := ioutil.NopCloser(strings.NewReader(c.Payload))

	agent_call, err := pr.a.GetCall(FromModelAndInput(&c, player),
		WithWriter(recorder),
		WithContext(execCtx),
	)

	if err == nil {
		var mcall *call
		mcall = agent_call.(*call)
		err = pr.a.Submit(mcall)
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
				// This is because call.End() is executed asynchronously.
				result.CompletedAt = common.DateTime(time.Now()).String()
			}
		}
	}

	// Status images should not output excessive data since we echo the
	// data back to caller.
	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	// Clamp the log output to 256 bytes if output is too large for logging.
	dLen := len(body)
	if dLen > 256 {
		dLen = 256
	}
	log.Debugf("Status call with id=%v result=%+v body[0:%v]=%v", c.ID, result, dLen, string(body[:dLen]))

	result.Details = string(body)
	result.Id = c.ID
	return result
}

// Handles a status call concurrency and caching.
func (pr *pureRunner) handleStatusCall(ctx context.Context) (*runner.RunnerStatus, error) {
	var myChan chan struct{}

	isWaiter := false
	isCached := false
	now := time.Now()

	pr.status.lock.Lock()

	if now.Before(pr.status.expiry) {
		// cache is still valid.
		isCached = true
	} else if pr.status.wait != nil {
		// A wait channel is already installed, we must wait
		isWaiter = true
		myChan = pr.status.wait
	} else {
		// Wait channel is not present, we install a new one.
		myChan = make(chan struct{})
		pr.status.wait = myChan
	}

	pr.status.lock.Unlock()

	// We either need to wait and/or serve the request from cache
	if isWaiter || isCached {
		if isWaiter {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-myChan:
			}
		}

		var cacheObj runner.RunnerStatus

		// A shallow copy is sufficient here, as we do not modify nested data in
		// RunnerStatus in any way.
		pr.status.lock.Lock()

		cacheObj = *pr.status.cache

		pr.status.lock.Unlock()

		cacheObj.Cached = true
		cacheObj.Active = atomic.LoadInt32(&pr.status.inflight)
		cacheObj.RequestsReceived = atomic.LoadUint64(&pr.status.requestsReceived)
		cacheObj.RequestsHandled = atomic.LoadUint64(&pr.status.requestsHandled)
		return &cacheObj, nil
	}

	cachePtr := pr.runStatusCall(ctx)
	cachePtr.Active = atomic.LoadInt32(&pr.status.inflight)
	cachePtr.RequestsReceived = atomic.LoadUint64(&pr.status.requestsReceived)
	cachePtr.RequestsHandled = atomic.LoadUint64(&pr.status.requestsHandled)
	now = time.Now()

	// Pointer store of 'cachePtr' is sufficient here as isWaiter/isCached above perform a shallow
	// copy of 'cache'
	pr.status.lock.Lock()

	pr.status.cache = cachePtr
	pr.status.expiry = now.Add(StatusCallCacheDuration)
	pr.status.wait = nil

	pr.status.lock.Unlock()

	// signal waiters
	close(myChan)
	return cachePtr, nil
}

// implements RunnerProtocolServer
func (pr *pureRunner) Status(ctx context.Context, _ *empty.Empty) (*runner.RunnerStatus, error) {
	// Status using image name is disabled. We return inflight request count only
	if pr.status.imageName == "" {
		return &runner.RunnerStatus{
			Active:           atomic.LoadInt32(&pr.status.inflight),
			RequestsReceived: atomic.LoadUint64(&pr.status.requestsReceived),
			RequestsHandled:  atomic.LoadUint64(&pr.status.requestsHandled),
		}, nil
	}
	return pr.handleStatusCall(ctx)
}

func DefaultPureRunner(cancel context.CancelFunc, addr string, da CallHandler, tlsCfg *tls.Config) (Agent, error) {

	agent := New(da)

	// WARNING: SSL creds are optional.
	if tlsCfg == nil {
		return NewPureRunner(cancel, addr, PureRunnerWithAgent(agent))
	}
	return NewPureRunner(cancel, addr, PureRunnerWithAgent(agent), PureRunnerWithSSL(tlsCfg))
}

type PureRunnerOption func(*pureRunner) error

func PureRunnerWithSSL(tlsCfg *tls.Config) PureRunnerOption {
	return func(pr *pureRunner) error {
		pr.creds = credentials.NewTLS(tlsCfg)
		return nil
	}
}

func PureRunnerWithAgent(a Agent) PureRunnerOption {
	return func(pr *pureRunner) error {
		if pr.a != nil {
			return errors.New("Failed to create pure runner: agent already created")
		}

		pr.a = a
		return nil
	}
}

// PureRunnerWithStatusImage returns a PureRunnerOption that annotates a PureRunner with a
// statusImageName attribute.  This attribute names an image name to use for the status checks.
// Optionally, the status image can be pre-loaded into docker using FN_DOCKER_LOAD_FILE to avoid
// docker pull during status checks.
func PureRunnerWithStatusImage(imgName string) PureRunnerOption {
	return func(pr *pureRunner) error {
		if pr.status.imageName != "" {
			return fmt.Errorf("Duplicate status image configuration old=%s new=%s", pr.status.imageName, imgName)
		}
		pr.status.imageName = imgName
		return nil
	}
}

func NewPureRunner(cancel context.CancelFunc, addr string, options ...PureRunnerOption) (Agent, error) {

	pr := &pureRunner{}

	for _, option := range options {
		err := option(pr)
		if err != nil {
			logrus.WithError(err).Fatalf("error in pure runner options")
		}
	}

	if pr.a == nil {
		logrus.Fatal("agent not provided in pure runner options")
	}

	var opts []grpc.ServerOption

	opts = append(opts, grpc.StreamInterceptor(grpcutil.RIDStreamServerInterceptor))
	opts = append(opts, grpc.UnaryInterceptor(grpcutil.RIDUnaryServerInterceptor))

	if pr.creds != nil {
		opts = append(opts, grpc.Creds(pr.creds))
	} else {
		logrus.Warn("Running pure runner in insecure mode!")
	}

	pr.gRPCServer = grpc.NewServer(opts...)
	runner.RegisterRunnerProtocolServer(pr.gRPCServer, pr)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logrus.WithError(err).Fatalf("Could not listen on %s", addr)
	}

	logrus.Info("Pure Runner listening on ", addr)

	go func() {
		if err := pr.gRPCServer.Serve(lis); err != nil {
			logrus.WithError(err).Error("grpc serve error")
			cancel()
		}
	}()

	return pr, nil
}

var _ runner.RunnerProtocolServer = &pureRunner{}
var _ Agent = &pureRunner{}
