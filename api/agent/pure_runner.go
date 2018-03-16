package agent

import (
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

// callHandle represents the state of the call as handled by the pure runner, and additionally it implements the
// interface of http.ResponseWriter so that it can be used for streaming the output back.
type callHandle struct {
	engagement runner.RunnerProtocol_EngageServer
	c          *call // the agent's version of call
	input      io.WriteCloser
	started    bool
	done       chan error // to synchronize
	// As the state can be set and checked by both goroutines handling this state, we need a mutex.
	stateMutex sync.Mutex
	// Timings, for metrics:
	receivedTime  strfmt.DateTime // When was the call received?
	allocatedTime strfmt.DateTime // When did we finish allocating capacity?
	// Last communication error on the stream (if any). This basically acts as a cancellation flag too.
	streamError error
	// For implementing http.ResponseWriter:
	outHeaders    http.Header
	outStatus     int
	headerWritten bool
}

func (ch *callHandle) Header() http.Header {
	return ch.outHeaders
}

func (ch *callHandle) WriteHeader(status int) {
	ch.outStatus = status
	ch.commitHeaders()
}

func (ch *callHandle) commitHeaders() error {
	if ch.headerWritten {
		return nil
	}
	ch.headerWritten = true
	logrus.Debugf("Committing call result with status %d", ch.outStatus)

	var outHeaders []*runner.HttpHeader

	for h, vals := range ch.outHeaders {
		for _, v := range vals {
			outHeaders = append(outHeaders, &runner.HttpHeader{
				Key:   h,
				Value: v,
			})
		}
	}

	// Only write if we are not in an error situation. If we cause a stream error, then record that but don't cancel
	// the call: basically just blackhole the output and return the write error to cause Submit to fail properly.
	ch.stateMutex.Lock()
	defer ch.stateMutex.Unlock()
	err := ch.streamError
	if err != nil {
		return fmt.Errorf("Bailing out because of communication error: %v", ch.streamError)
	}

	logrus.Debug("Sending call result start message")
	err = ch.engagement.Send(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_ResultStart{
			ResultStart: &runner.CallResultStart{
				Meta: &runner.CallResultStart_Http{
					Http: &runner.HttpRespMeta{
						Headers:    outHeaders,
						StatusCode: int32(ch.outStatus),
					},
				},
			},
		},
	})
	if err != nil {
		logrus.WithError(err).Error("Error sending call result")
		ch.streamError = err
		return err
	}
	logrus.Debug("Sent call result message")
	return nil
}

func (ch *callHandle) Write(data []byte) (int, error) {
	err := ch.commitHeaders()
	if err != nil {
		return 0, fmt.Errorf("Error sending data: %v", err)
	}

	// Only write if we are not in an error situation. If we cause a stream error, then record that but don't cancel
	// the call: basically just blackhole the output and return the write error to cause Submit to fail properly.
	ch.stateMutex.Lock()
	defer ch.stateMutex.Unlock()
	err = ch.streamError
	if err != nil {
		return 0, fmt.Errorf("Bailing out because of communication error: %v", ch.streamError)
	}

	logrus.Debugf("Sending call response data %d bytes long", len(data))
	err = ch.engagement.Send(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Data: data,
				Eof:  false,
			},
		},
	})
	if err != nil {
		ch.streamError = err
		return 0, fmt.Errorf("Error sending data: %v", err)
	}
	return len(data), nil
}

func (ch *callHandle) Close() error {
	err := ch.commitHeaders()
	if err != nil {
		return fmt.Errorf("Error sending close frame: %v", err)
	}

	// Only write if we are not in an error situation. If we cause a stream error, then record that but don't cancel
	// the call: basically just blackhole the output and return the write error to cause the caller to fail properly.
	ch.stateMutex.Lock()
	defer ch.stateMutex.Unlock()
	err = ch.streamError
	if err != nil {
		return fmt.Errorf("Bailing out because of communication error: %v", ch.streamError)
	}
	logrus.Debug("Sending call response data end")
	err = ch.engagement.Send(&runner.RunnerMsg{
		Body: &runner.RunnerMsg_Data{
			Data: &runner.DataFrame{
				Eof: true,
			},
		},
	})

	if err != nil {
		return fmt.Errorf("Error sending close frame: %v", err)
	}
	return nil
}

// cancel implements the logic for cancelling the execution of a call based on what the state in the handle is.
func (ch *callHandle) cancel(ctx context.Context, err error) {
	ch.stateMutex.Lock()
	defer ch.stateMutex.Unlock()

	// Do not double-cancel.
	if ch.streamError != nil {
		return
	}

	// First, record that there has been an error.
	ch.streamError = err
	// Caller may have died or disconnected. The behaviour here depends on the state of the call.
	// If the call was placed and is running we need to handle it...
	if ch.c != nil {
		// If we've actually started the call we're in the middle of an execution with i/o going back and forth.
		// This is hard to stop. Side effects can be occurring at any point. However, at least we should stop
		// the i/o flow. Recording the stream error in the handle should have stopped the output, but we also
		// want to stop any input being sent through, so we close the input stream and let the function
		// probably crash out. If it doesn't crash out, well, it means the function doesn't handle i/o errors
		// properly and it will hang there until the timeout, then it'll be killed properly by the timeout
		// handling in Submit.
		if ch.started {
			ch.input.Close()
		}
	}
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

func (pr *pureRunner) ensureFunctionIsRunning(state *callHandle) {
	// Only start it once!
	state.stateMutex.Lock()
	defer state.stateMutex.Unlock()
	if !state.started {
		state.started = true
		go func() {
			err := pr.a.Submit(state.c)
			if err != nil {
				// In this case the function has failed for a legitimate reason. We send a call failed message if we
				// can. If there's a streaming error doing that then we are basically in the "double exception" case
				// and who knows what's best to do. Submit has already finished so we don't need to cancel... but at
				// least we should set streamError if it's not set.
				state.stateMutex.Lock()
				defer state.stateMutex.Unlock()
				if state.streamError == nil {
					err2 := state.engagement.Send(&runner.RunnerMsg{
						Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
							Success: false,
							Details: fmt.Sprintf("%v", err),
						}}})
					if err2 != nil {
						state.streamError = err2
					}
				}
				state.done <- err
				return
			}
			// First close the writer, then send the call finished message
			err = state.Close()
			if err != nil {
				// If we fail to close the writer we need to communicate back that the function has failed; if there's
				// a streaming error doing that then we are basically in the "double exception" case and who knows
				// what's best to do. Submit has already finished so we don't need to cancel... but at least we should
				// set streamError if it's not set.
				state.stateMutex.Lock()
				defer state.stateMutex.Unlock()
				if state.streamError == nil {
					err2 := state.engagement.Send(&runner.RunnerMsg{
						Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
							Success: false,
							Details: fmt.Sprintf("%v", err),
						}}})
					if err2 != nil {
						state.streamError = err2
					}
				}
				state.done <- err
				return
			}
			// At this point everything should have worked. Send a successful message... and if that runs afoul of a
			// stream error, well, we're in a bit of trouble. Everything has finished, so there is nothing to cancel
			// and we just give up, but at least we set streamError.
			state.stateMutex.Lock()
			defer state.stateMutex.Unlock()
			if state.streamError == nil {
				err2 := state.engagement.Send(&runner.RunnerMsg{
					Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
						Success: true,
						Details: state.c.Model().ID,
					}}})
				if err2 != nil {
					state.streamError = err2
					state.done <- err2
					return
				}
			}

			state.done <- nil
		}()
	}
}

func (pr *pureRunner) handleData(ctx context.Context, data *runner.DataFrame, state *callHandle) error {
	pr.ensureFunctionIsRunning(state)

	// Only push the input if we're in a non-error situation
	state.stateMutex.Lock()
	defer state.stateMutex.Unlock()
	if state.streamError == nil {
		if len(data.Data) > 0 {
			_, err := state.input.Write(data.Data)
			if err != nil {
				return err
			}
		}
		if data.Eof {
			state.input.Close()
		}
	}
	return nil
}

func (pr *pureRunner) handleTryCall(ctx context.Context, tc *runner.TryCall, state *callHandle) (capacityDeallocator, error) {
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

	// Proceed!
	var w http.ResponseWriter
	w = state
	inR, inW := io.Pipe()
	agent_call, err := pr.a.GetCall(FromModelAndInput(&c, inR), WithWriter(w))
	if err != nil {
		return func() { pr.capacity.ReleaseCapacity(c.Memory) }, err
	}
	state.c = agent_call.(*call)
	state.input = inW
	state.allocatedTime = strfmt.DateTime(time.Now())

	return func() { pr.capacity.ReleaseCapacity(c.Memory) }, nil
}

// Handles a client engagement
func (pr *pureRunner) Engage(engagement runner.RunnerProtocol_EngageServer) error {
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

	var state = callHandle{
		engagement:    engagement,
		c:             nil,
		input:         nil,
		started:       false,
		done:          make(chan error),
		streamError:   nil,
		outHeaders:    make(http.Header),
		outStatus:     200,
		headerWritten: false,
	}

	grpc.EnableTracing = false
	logrus.Debug("Entering engagement handler")

	msg, err := engagement.Recv()
	if err != nil {
		// In this case the connection has dropped before we've even started.
		return err
	}
	switch body := msg.Body.(type) {
	case *runner.ClientMsg_Try:
		dealloc, err := pr.handleTryCall(engagement.Context(), body.Try, &state)
		defer dealloc()
		// At the stage of TryCall, there is only one thread running and nothing has happened yet so there should
		// not be a streamError. We can handle `err` by sending a message back. If we cause a stream error by sending
		// the message, we are in a "double exception" case and we might as well cancel the call with the original
		// error, so we can ignore the error from Send.
		if err != nil {
			_ = engagement.Send(&runner.RunnerMsg{
				Body: &runner.RunnerMsg_Acknowledged{Acknowledged: &runner.CallAcknowledged{
					Committed: false,
					Details:   fmt.Sprintf("%v", err),
				}}})
			state.cancel(engagement.Context(), err)
			return err
		}

		// If we succeed in creating the call, but we get a stream error sending a message back, we must cancel
		// the call because we've probably lost the connection.
		err = engagement.Send(&runner.RunnerMsg{
			Body: &runner.RunnerMsg_Acknowledged{Acknowledged: &runner.CallAcknowledged{
				Committed:             true,
				Details:               state.c.Model().ID,
				SlotAllocationLatency: time.Time(state.allocatedTime).Sub(time.Time(state.receivedTime)).String(),
			}}})
		if err != nil {
			state.cancel(engagement.Context(), err)
			return err
		}

		// Then at this point we start handling the data that should be being pushed to us.
		foundEof := false
		for !foundEof {
			msg, err := engagement.Recv()
			if err != nil {
				// In this case the connection has dropped or there's something bad happening. We know we can't even
				// send a message back. Cancel the call, all bets are off.
				state.cancel(engagement.Context(), err)
				return err
			}

			switch body := msg.Body.(type) {
			case *runner.ClientMsg_Data:
				err := pr.handleData(engagement.Context(), body.Data, &state)
				if err != nil {
					// If this happens, then we couldn't write into the input. The state of the function is inconsistent
					// and therefore we need to cancel. We also need to communicate back that the function has failed;
					// that could also run afoul of a stream error, but at that point we don't care, just cancel the
					// call with the original error.
					_ = state.engagement.Send(&runner.RunnerMsg{
						Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
							Success: false,
							Details: fmt.Sprintf("%v", err),
						}}})
					state.cancel(engagement.Context(), err)
					return err
				}
				// Then break the loop if this was the last input data frame, i.e. eof is on
				if body.Data.Eof {
					foundEof = true
				}
			default:
				err := errors.New("Protocol failure in communication with function runner")
				// This is essentially a panic. Try to communicate back that the call has failed, and bail out; that
				// could also run afoul of a stream error, but at that point we don't care, just cancel the call with
				// the catastrophic error.
				_ = state.engagement.Send(&runner.RunnerMsg{
					Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
						Success: false,
						Details: fmt.Sprintf("%v", err),
					}}})
				state.cancel(engagement.Context(), err)
				return err
			}
		}

		// Synchronize to the function running goroutine finishing
		select {
		case <-state.done:
		case <-engagement.Context().Done():
			return engagement.Context().Err()
		}

	default:
		// Protocol error. This should not happen.
		return errors.New("Protocol failure in communication with function runner")
	}

	return nil
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

func UnsecuredPureRunner(cancel context.CancelFunc, addr string, a Agent) (*pureRunner, error) {
	return NewPureRunner(cancel, addr, a, "", "", "", nil)
}

func DefaultPureRunner(cancel context.CancelFunc, addr string, a Agent, cert string, key string, ca string) (*pureRunner, error) {
	return NewPureRunner(cancel, addr, a, cert, key, ca, nil)
}

func NewPureRunner(cancel context.CancelFunc, addr string, a Agent, cert string, key string, ca string, gate CapacityGate) (*pureRunner, error) {
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
