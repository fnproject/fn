package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"sync/atomic"
	"time"

	"github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/event"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/fnproject/fn/grpcutil"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"reflect"
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

//// pureRunner implements Agent and delegates execution of functions to an internal Agent; basically it wraps around it
//// and provides the gRPC server that implements the LB <-> Runner protocol.
type pureRunner struct {
	gRPCServer *grpc.Server
	creds      credentials.TransportCredentials
	a          Agent
	inflight   int32
}

// implements Agent
func (pr *pureRunner) GetCall(ctx context.Context, opts ...CallOpt) (Call, error) {
	return pr.a.GetCall(ctx, opts...)
}

// implements Agent
func (pr *pureRunner) Submit(context.Context, Call) (*event.Event, error) {
	return nil, errors.New("Submit cannot be called directly in a Pure Runner.")
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

var (
	ErrInvaldInitMessage = errors.New("unexpected opening message type  wanted TryCall ")
	ErrInvaldDataMessage = errors.New("unexpected Data message type  wanted TryCall ")
	ErrBodySizeToLarge   = errors.New("body size exceeds maximum")
	ErrInvalidPayload    = errors.New("payload was not parsable as JSON")
)

// TODO configize
const (
	MaxBodySize = uint64(1024 * 1024)
)

func createSubmitResponse(mcall *models.Call, err error) *runner.RunnerMsg {

	var createdAt string
	var startedAt string
	var completedAt string
	var details string
	var errCode int
	var errStr string

	if err != nil {
		errCode = models.GetAPIErrorCode(err)
		errStr = err.Error()
	}

	if mcall != nil {

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

	return &runner.RunnerMsg{
		Body: &runner.RunnerMsg_Finished{Finished: &runner.CallFinished{
			Success:     err == nil,
			Details:     details,
			ErrorCode:   int32(errCode),
			ErrorStr:    errStr,
			CreatedAt:   createdAt,
			StartedAt:   startedAt,
			CompletedAt: completedAt,
		}}}

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

	msg, err := engagement.Recv()
	if err != nil {
		return fmt.Errorf("error receiving trycall: %s", err)
	}

	tcm, ok := msg.GetBody().(*runner.ClientMsg_Try)
	if !ok {
		log.Error("expecting a tryCall message to open dialog, got a %s", reflect.TypeOf(msg.GetBody()))
		return ErrInvaldInitMessage
	}
	tryMsg := tcm.Try

	var c models.Call
	err = json.Unmarshal([]byte(tryMsg.ModelsCallJson), &c)

	if err != nil {
		return fmt.Errorf("invalid JSON call body %s", err)
	}

	// IMPORTANT: We clear/initialize these dates as start/created/completed dates from
	// unmarshalled Model from LB-agent represent unrelated time-line events.
	// From this point, CreatedAt/StartedAt/CompletedAt are based on our local clock.
	start := time.Now()
	c.CreatedAt = common.DateTime(start)
	c.StartedAt = common.DateTime(time.Time{})
	c.CompletedAt = common.DateTime(time.Time{})

	// TODO buffer pool here
	bodyBuf := &bytes.Buffer{}
	bodyWriter := common.NewClampWriter(bodyBuf, MaxBodySize, ErrBodySizeToLarge)

	for {
		msg, err := engagement.Recv()
		if err != nil {
			return err
		}

		dfms, ok := msg.Body.(*runner.ClientMsg_Data)
		if !ok {
			log.Errorf("Got unexpected message from client %s", reflect.TypeOf(msg.Body))
			return ErrInvaldDataMessage
		}
		_, err = bodyWriter.Write(dfms.Data.Data)
		if err != nil {
			return err
		}

		if dfms.Data.Eof {
			break
		}
	}

	var inputEvt event.Event
	err = json.NewDecoder(bytes.NewReader(bodyBuf.Bytes())).Decode(&inputEvt)

	if err != nil {
		log.WithError(err).Error("Invalid JSON payload ")
		return ErrInvalidPayload
	}

	c.InputEvent = &inputEvt

	agentCall, err := pr.a.GetCall(ctx, FromModel(&c), WithExtensions(tryMsg.Extensions))
	if err != nil {
		return err
	}

	// TODO - this seems odd/wrong - canonical call ID should be part of the call model
	if tryMsg.SlotHashId != "" {
		hashID, err := hex.DecodeString(tryMsg.SlotHashId)
		if err != nil {
			return err
		}
		agentCall.(*call).slotHashId = string(hashID[:])
	}

	resp, err := pr.a.Submit(ctx, agentCall)

	if err != nil {
		return engagement.Send(createSubmitResponse(&c, err))
	}

	respBuf := &bytes.Buffer{}
	err = json.NewEncoder(respBuf).Encode(resp)
	if err != nil {
		return err
	}

	respBytes := respBuf.Bytes()
	// Now messages are fully buffered there isn't much reason to buffer any more
	for offset := 0; offset < len(respBytes); offset += MaxDataChunk {
		top := offset + MaxDataChunk
		eof := false
		if top >= len(respBytes) {
			eof = true
			top = len(respBytes)
		}
		err = engagement.Send(&runner.RunnerMsg{
			Body: &runner.RunnerMsg_Data{
				Data: &runner.DataFrame{
					Data: respBytes[offset:top],
					Eof:  eof,
				},
			},
		})

	}
	//
	return engagement.Send(createSubmitResponse(&c, nil))
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

func DefaultPureRunner(cancel context.CancelFunc, addr string, da CallHandler, cert string, key string, ca string) (Agent, error) {

	agent := New(da)

	// WARNING: SSL creds are optional.
	if cert == "" || key == "" || ca == "" {
		return NewPureRunner(cancel, addr, PureRunnerWithAgent(agent))
	}
	return NewPureRunner(cancel, addr, PureRunnerWithAgent(agent), PureRunnerWithSSL(cert, key, ca))
}

type PureRunnerOption func(*pureRunner) error

func PureRunnerWithSSL(cert string, key string, ca string) PureRunnerOption {
	return func(pr *pureRunner) error {
		c, err := createCreds(cert, key, ca)
		if err != nil {
			return fmt.Errorf("Failed to create pure runner credentials: %s", err)
		}
		pr.creds = c
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
		logrus.WithError(err).Fatalf("could not listen on %s", addr)
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

func createCreds(cert string, key string, ca string) (credentials.TransportCredentials, error) {
	if cert == "" || key == "" || ca == "" {
		return nil, errors.New("failed to create credentials, cert/key/ca not provided")
	}

	// Load the certificates from disk
	certificate, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("could not load server key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	authority, err := ioutil.ReadFile(ca)
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}

	if ok := certPool.AppendCertsFromPEM(authority); !ok {
		return nil, errors.New("failed to append client certs")
	}

	return credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	}), nil
}

var _ runner.RunnerProtocolServer = &pureRunner{}
var _ Agent = &pureRunner{}
