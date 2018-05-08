package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fnproject/cloudevent"
	runner "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/go-openapi/strfmt"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
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

	// Timings, for metrics:
	receivedTime  strfmt.DateTime // When was the call received?
	allocatedTime strfmt.DateTime // When did we finish allocating capacity?

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
		logrus.WithError(err).Debugf("Shutting down call handle")

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

// enqueueCallResponse enqueues a Submit() response to the LB
// and initiates a graceful shutdown of the session.
func (ch *callHandle) enqueueCallResponse(err error) {
	defer ch.finalize()

	// Error response
	if err != nil {
		ch.enqueueMsgStrict(&runner.RunnerMsg{
			Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
				Success: false,
				Details: fmt.Sprintf("%v", err),
			}}})
		return
	}

	// EOF and Success response
	ch.enqueueMsgStrict(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Eof: true,
			},
		},
	})

	ch.enqueueMsgStrict(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
			Success: true,
			Details: ch.c.Model().ID,
		}}})
}

// enqueueAck enqueues a ACK or NACK response to the LB for ClientMsg_Try
// request. If NACK, then it also initiates a graceful shutdown of the
// session.
func (ch *callHandle) enqueueAck(err error) error {
	// NACK
	if err != nil {
		err = ch.enqueueMsgStrict(&runner.RunnerMsg{
			Body: &runner.RunnerMsg_Acknowledged{Acknowledged: &runner.CallAcknowledged{
				Committed: false,
				Details:   fmt.Sprintf("%v", err),
			}}})
		if err != nil {
			return err
		}
		return ch.finalize()
	}

	// ACK
	return ch.enqueueMsgStrict(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Acknowledged{Acknowledged: &runner.CallAcknowledged{
			Committed:             true,
			Details:               ch.c.Model().ID,
			SlotAllocationLatency: time.Time(ch.allocatedTime).Sub(time.Time(ch.receivedTime)).String(),
		}}})
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

	// we cannot retain 'data'
	cpData := make([]byte, len(data))
	copy(cpData, data)

	err = ch.enqueueMsg(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Data: cpData,
				Eof:  false,
			},
		},
	})

	if err != nil {
		return 0, err
	}
	return len(data), nil
}

// getTryMsg fetches/waits for a TryCall message from
// the LB using inQueue (gRPC receiver)
func (ch *callHandle) getTryMsg() *runner.TryCall {
	var msg *runner.TryCall

	select {
	case <-ch.doneQueue:
	case <-ch.ctx.Done():
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

type CapacityGate interface {
	// CheckAndReserveCapacity must perform an atomic check plus reservation. If an error is returned, then it is
	// guaranteed that no capacity has been committed. If nil is returned, then it is guaranteed that the provided units
	// of capacity have been committed.
	CheckAndReserveCapacity(units uint64) error

	// ReleaseCapacity must perform an atomic release of capacity. The units provided must not bring the capacity under
	// zero; implementations are free to panic in that case.
	ReleaseCapacity(units uint64)
}

type pureRunnerCapacityManager struct {
	totalCapacityUnits     uint64
	committedCapacityUnits uint64
	mtx                    sync.Mutex
}

type capacityDeallocator func()

func newPureRunnerCapacityManager(units uint64) *pureRunnerCapacityManager {
	return &pureRunnerCapacityManager{
		totalCapacityUnits:     units,
		committedCapacityUnits: 0,
	}
}

func (prcm *pureRunnerCapacityManager) CheckAndReserveCapacity(units uint64) error {
	prcm.mtx.Lock()
	defer prcm.mtx.Unlock()
	if prcm.totalCapacityUnits-prcm.committedCapacityUnits >= units {
		prcm.committedCapacityUnits = prcm.committedCapacityUnits + units
		return nil
	}
	return models.ErrCallTimeoutServerBusy
}

func (prcm *pureRunnerCapacityManager) ReleaseCapacity(units uint64) {
	prcm.mtx.Lock()
	defer prcm.mtx.Unlock()
	if units <= prcm.committedCapacityUnits {
		prcm.committedCapacityUnits = prcm.committedCapacityUnits - units
		return
	}
	panic("Fatal error in pure runner capacity calculation, getting to sub-zero capacity")
}

// pureRunner implements Agent and delegates execution of functions to an internal Agent; basically it wraps around it
// and provides the gRPC server that implements the LB <-> Runner protocol.
type pureRunner struct {
	gRPCServer *grpc.Server
	listen     string
	a          Agent
	inflight   int32
	capacity   CapacityGate
}

func (pr *pureRunner) GetAppID(ctx context.Context, appName string) (string, error) {
	return pr.a.GetAppID(ctx, appName)
}

func (pr *pureRunner) GetAppByID(ctx context.Context, appID string) (*models.App, error) {
	return pr.a.GetAppByID(ctx, appID)
}

func (pr *pureRunner) GetCall(opts ...CallOpt) (Call, error) {
	return pr.a.GetCall(opts...)
}

func (pr *pureRunner) GetRoute(ctx context.Context, appID string, path string) (*models.Route, error) {
	return pr.a.GetRoute(ctx, appID, path)
}

func (pr *pureRunner) Submit(Call) error {
	return errors.New("Submit cannot be called directly in a Pure Runner.")
}

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

func (pr *pureRunner) Handle(ctx context.Context, event cloudevent.CloudEvent) (cloudevent.CloudEvent, error) {
	// TODO
	return cloudevent.CloudEvent{}, errors.New("Handle not implemented")
}

func (pr *pureRunner) AddCallListener(cl fnext.CallListener) {
	pr.a.AddCallListener(cl)
}

func (pr *pureRunner) Enqueue(context.Context, *models.Call) error {
	return errors.New("Enqueue cannot be called directly in a Pure Runner.")
}

func (pr *pureRunner) spawnSubmit(state *callHandle) {
	go func() {
		err := pr.a.Submit(state.c)
		state.enqueueCallResponse(err)
	}()
}

// handleTryCall based on the TryCall message, allocates a resource/capacity reservation
// and creates callHandle.c with agent.call.
func (pr *pureRunner) handleTryCall(tc *runner.TryCall, state *callHandle) (capacityDeallocator, error) {
	state.receivedTime = strfmt.DateTime(time.Now())
	var c models.Call
	err := json.Unmarshal([]byte(tc.ModelsCallJson), &c)
	if err != nil {
		return func() {}, err
	}

	// Capacity check first
	err = pr.capacity.CheckAndReserveCapacity(c.Memory)
	if err != nil {
		return func() {}, err
	}

	cleanup := func() {
		pr.capacity.ReleaseCapacity(c.Memory)
	}

	agent_call, err := pr.a.GetCall(FromModelAndInput(&c, state.pipeToFnR), WithWriter(state))
	if err != nil {
		return cleanup, err
	}

	state.c = agent_call.(*call)
	state.allocatedTime = strfmt.DateTime(time.Now())

	return cleanup, nil
}

// Handles a client engagement
func (pr *pureRunner) Engage(engagement runner.RunnerProtocol_EngageServer) error {
	grpc.EnableTracing = false

	// Keep lightweight tabs on what this runner is doing: for draindown tests
	atomic.AddInt32(&pr.inflight, 1)
	defer atomic.AddInt32(&pr.inflight, -1)

	pv, ok := peer.FromContext(engagement.Context())
	logrus.Debug("Starting engagement")
	if ok {
		logrus.Debug("Peer is ", pv)
	}
	md, ok := metadata.FromIncomingContext(engagement.Context())
	if ok {
		logrus.Debug("MD is ", md)
	}

	state := NewCallHandle(engagement)

	tryMsg := state.getTryMsg()
	if tryMsg == nil {
		return state.waitError()
	}

	dealloc, errTry := pr.handleTryCall(tryMsg, state)
	defer dealloc()
	// respond with handleTryCall response
	err := state.enqueueAck(errTry)
	if err != nil || errTry != nil {
		return state.waitError()
	}

	var dataFeed chan *runner.DataFrame
DataLoop:
	for {
		dataMsg := state.getDataMsg()
		if dataMsg == nil {
			break
		}

		if dataFeed == nil {
			pr.spawnSubmit(state)
			dataFeed = state.spawnPipeToFn()
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

	return state.waitError()
}

func (pr *pureRunner) Status(ctx context.Context, _ *empty.Empty) (*runner.RunnerStatus, error) {
	return &runner.RunnerStatus{
		Active: atomic.LoadInt32(&pr.inflight),
	}, nil
}

func (pr *pureRunner) Start() error {
	logrus.Info("Pure Runner listening on ", pr.listen)
	lis, err := net.Listen("tcp", pr.listen)
	if err != nil {
		return fmt.Errorf("Could not listen on %s: %s", pr.listen, err)
	}

	if err := pr.gRPCServer.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve error: %s", err)
	}
	return err
}

func UnsecuredPureRunner(cancel context.CancelFunc, addr string, da DataAccess) (Agent, error) {
	return NewPureRunner(cancel, addr, da, "", "", "", nil)
}

func DefaultPureRunner(cancel context.CancelFunc, addr string, da DataAccess, cert string, key string, ca string) (Agent, error) {
	return NewPureRunner(cancel, addr, da, cert, key, ca, nil)
}

func NewPureRunner(cancel context.CancelFunc, addr string, da DataAccess, cert string, key string, ca string, gate CapacityGate) (Agent, error) {
	a := createAgent(da)
	var pr *pureRunner
	var err error
	if cert != "" && key != "" && ca != "" {
		c, err := creds(cert, key, ca)
		if err != nil {
			logrus.WithField("runner_addr", addr).Warn("Failed to create credentials!")
			return nil, err
		}
		pr, err = createPureRunner(addr, a, c, gate)
		if err != nil {
			return nil, err
		}
	} else {
		logrus.Warn("Running pure runner in insecure mode!")
		pr, err = createPureRunner(addr, a, nil, gate)
		if err != nil {
			return nil, err
		}
	}

	go func() {
		err := pr.Start()
		if err != nil {
			logrus.WithError(err).Error("Failed to start pure runner")
			cancel()
		}
	}()

	return pr, nil
}

func creds(cert string, key string, ca string) (credentials.TransportCredentials, error) {
	// Load the certificates from disk
	certificate, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("Could not load server key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	authority, err := ioutil.ReadFile(ca)
	if err != nil {
		return nil, fmt.Errorf("Could not read ca certificate: %s", err)
	}

	if ok := certPool.AppendCertsFromPEM(authority); !ok {
		return nil, errors.New("Failed to append client certs")
	}

	return credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	}), nil
}

func createPureRunner(addr string, a Agent, creds credentials.TransportCredentials, gate CapacityGate) (*pureRunner, error) {
	var srv *grpc.Server
	if creds != nil {
		srv = grpc.NewServer(grpc.Creds(creds))
	} else {
		srv = grpc.NewServer()
	}
	if gate == nil {
		memUnits := getAvailableMemoryUnits()
		gate = newPureRunnerCapacityManager(memUnits)
	}
	pr := &pureRunner{
		gRPCServer: srv,
		listen:     addr,
		a:          a,
		capacity:   gate,
	}

	runner.RegisterRunnerProtocolServer(srv, pr)
	return pr, nil
}

const megabyte uint64 = 1024 * 1024

func getAvailableMemoryUnits() uint64 {
	// To reuse code - but it's a bit of a hack. TODO: refactor the OS-specific get memory funcs out of that.
	throwawayRT := NewResourceTracker(nil).(*resourceTracker)
	return throwawayRT.ramAsyncTotal / megabyte
}
