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
	pipeToFn  io.WriteCloser
}

func NewCallHandle(engagement runner.RunnerProtocol_EngageServer) *callHandle {
	state := &callHandle{
		engagement: engagement,
		ctx:        engagement.Context(),
		headers:    make(http.Header),
		status:     200,
		outQueue:   make(chan *runner.RunnerMsg),
		doneQueue:  make(chan struct{}),
		errQueue:   make(chan error, 1), // always allow one error (buffered)
		inQueue:    make(chan *runner.ClientMsg),
	}

	// spawn receiver and sender go-routines. We can work
	// concurrently on engagement Send()/Recv() separately, but multiple
	// go-routines cannot issue Send() at the same time.
	state.spawnReceiver()
	state.spawnSender()
	return state
}

func (ch *callHandle) closePipeToFn() {
	ch.pipeToFnCloseOnce.Do(func() {
		if ch.pipeToFn != nil {
			ch.pipeToFn.Close()
		}
	})
}

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

func (ch *callHandle) enqueueMsg(msg *runner.RunnerMsg) error {
	select {
	case ch.outQueue <- msg:
		return nil
	case <-ch.ctx.Done():
	case <-ch.doneQueue:
	}
	return io.EOF
}

func (ch *callHandle) enqueueCallResponse(err error) {

	if err != nil {
		err := ch.enqueueMsg(&runner.RunnerMsg{
			Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
				Success: false,
				Details: fmt.Sprintf("%v", err),
			}}})
		if err != nil {
			ch.shutdown(err)
			return
		}
	}

	err = ch.enqueueMsg(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Eof: true,
			},
		},
	})
	if err != nil {
		ch.shutdown(err)
		return
	}

	err = ch.enqueueMsg(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
			Success: true,
			Details: ch.c.Model().ID,
		}}})
	if err != nil {
		ch.shutdown(err)
		return
	}

	// final sentinel nil msg
	err = ch.enqueueMsg(nil)
	if err != nil {
		ch.shutdown(err)
		return
	}
}

func (ch *callHandle) enqueueAck(err error) error {

	// NACK
	if err != nil {
		msg := &runner.RunnerMsg{
			Body: &runner.RunnerMsg_Acknowledged{Acknowledged: &runner.CallAcknowledged{
				Committed: false,
				Details:   fmt.Sprintf("%v", err),
			}}}

		err = ch.enqueueMsg(msg)
		if err != nil {
			ch.shutdown(err)
			return err
		}
		err = ch.enqueueMsg(nil)
		if err != nil {
			ch.shutdown(err)
			return err
		}
		return nil
	}

	// ACK
	msg := &runner.RunnerMsg{
		Body: &runner.RunnerMsg_Acknowledged{Acknowledged: &runner.CallAcknowledged{
			Committed:             true,
			Details:               ch.c.Model().ID,
			SlotAllocationLatency: time.Time(ch.allocatedTime).Sub(time.Time(ch.receivedTime)).String(),
		}}}

	err = ch.enqueueMsg(msg)
	if err != nil {
		ch.shutdown(err)
	}
	return err
}

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
					_, err := io.CopyN(ch.pipeToFn, bytes.NewReader(data.Data), int64(len(data.Data)))
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

func (ch *callHandle) spawnReceiver() {

	go func() {
		defer close(ch.inQueue)
		for {
			msg, err := ch.engagement.Recv()
			if err != nil {
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

func (ch *callHandle) Header() http.Header {
	return ch.headers
}

func (ch *callHandle) WriteHeader(status int) {
	ch.status = status
}

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

func (ch *callHandle) Write(data []byte) (int, error) {
	var err error
	ch.headerOnce.Do(func() {
		// WARNING: we do fetch Status and Headers without
		// a lock below. This is a problem in agent in general, and needs
		// to be fixed in all accessing go-routines such as protocol/http.go,
		// protocol/json.go, agent.go, etc. In practice however, one go routine
		// accesses them (which also compiles and writes headers), but this
		// is fragile and needs to be fortified.
		msg := &runner.RunnerMsg{
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
		}

		err = ch.enqueueMsg(msg)
	})

	if err != nil {
		return 0, err
	}

	msg := &runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Data: data,
				Eof:  false,
			},
		},
	}

	err = ch.enqueueMsg(msg)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

func (ch *callHandle) getTryMsg() *runner.TryCall {
	var msg *runner.TryCall

	select {
	case <-ch.doneQueue:
	case <-ch.ctx.Done():
	case item := <-ch.inQueue:
		if item != nil {
			msg = item.GetTry()
			if msg == nil {
				ch.shutdown(ErrorExpectedTry)
			}
		}
	}
	return msg
}

func (ch *callHandle) getDataMsg() *runner.DataFrame {
	var msg *runner.DataFrame

	select {
	case <-ch.doneQueue:
	case <-ch.ctx.Done():
	case item := <-ch.inQueue:
		if item != nil {
			msg = item.GetData()
			if msg == nil {
				ch.shutdown(ErrorExpectedData)
			}
		}
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

	// Proceed!
	var w http.ResponseWriter
	w = state
	inR, inW := io.Pipe()
	state.pipeToFn = inW
	agent_call, err := pr.a.GetCall(FromModelAndInput(&c, inR), WithWriter(w))
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
		case <-state.doneQueue:
			break
		case <-state.ctx.Done():
			break
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
	a := createAgent(da, true)
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
