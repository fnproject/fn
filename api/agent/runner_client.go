package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/common"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/grpcutil"
	"github.com/sirupsen/logrus"
)

var (
	ErrorRunnerClosed   = errors.New("Runner is closed")
	ErrorPureRunnerNACK = errors.New("Purerunner NACK response")
)

const (
	// max buffer size for grpc data messages, 64K
	MaxDataChunk = 64 * 1024
)

type gRPCRunner struct {
	shutWg  *common.WaitGroup
	address string
	conn    *grpc.ClientConn
	client  pb.RunnerProtocolClient
}

func SecureGRPCRunnerFactory(addr, runnerCertCN string, pki *pool.PKIData) (pool.Runner, error) {
	conn, client, err := runnerConnection(addr, runnerCertCN, pki)
	if err != nil {
		return nil, err
	}

	return &gRPCRunner{
		shutWg:  common.NewWaitGroup(),
		address: addr,
		conn:    conn,
		client:  client,
	}, nil
}

func (r *gRPCRunner) Close(context.Context) error {
	r.shutWg.CloseGroup()
	return r.conn.Close()
}

func runnerConnection(address, runnerCertCN string, pki *pool.PKIData) (*grpc.ClientConn, pb.RunnerProtocolClient, error) {
	ctx := context.Background()

	var creds credentials.TransportCredentials
	if pki != nil {
		var err error
		creds, err = grpcutil.CreateCredentials(pki.Cert, pki.Key, pki.Ca, runnerCertCN)
		if err != nil {
			logrus.WithError(err).Error("Unable to create credentials to connect to runner node")
			return nil, nil, err
		}
	}

	// we want to set a very short timeout to fail-fast if something goes wrong
	conn, err := grpcutil.DialWithBackoff(ctx, address, creds, 100*time.Millisecond, grpc.DefaultBackoffConfig)
	if err != nil {
		logrus.WithError(err).Error("Unable to connect to runner node")
	}

	protocolClient := pb.NewRunnerProtocolClient(conn)
	logrus.WithField("runner_addr", address).Info("Connected to runner")

	return conn, protocolClient, nil
}

func (r *gRPCRunner) Address() string {
	return r.address
}

func (r *gRPCRunner) TryExec(ctx context.Context, call pool.RunnerCall) (bool, error) {
	logrus.WithField("runner_addr", r.address).Debug("Attempting to place call")
	if !r.shutWg.AddSession(1) {
		return true, ErrorRunnerClosed
	}
	defer r.shutWg.DoneSession()

	// extract the call's model data to pass on to the pure runner
	modelJSON, err := json.Marshal(call.Model())
	if err != nil {
		logrus.WithError(err).Error("Failed to encode model as JSON")
		// If we can't encode the model, no runner will ever be able to run this. Give up.
		return true, err
	}
	runnerConnection, err := r.client.Engage(ctx)
	if err != nil {
		logrus.WithError(err).Error("Unable to create client to runner node")
		// Try on next runner
		return false, err
	}

	// After this point, we assume "COMMITTED" unless pure runner
	// send explicit NACK

	recvDone := make(chan error, 1)
	sendDone := make(chan error, 1)

	// for safety, let's check if this call object is CachedReader. LB should
	// use a CachedReader to allow retries, but let's guard against
	// tools (runner-ping, etc) for compatibility.
	cachedReader, isCachedReader := call.RequestBody().(common.CachedReader)
	if isCachedReader {
		// for potential retries, let's reset our cached reader
		cachedReader.Reset()
	}

	go receiveFromRunner(runnerConnection, call, recvDone)
	go sendToRunner(runnerConnection, call, modelJSON, sendDone)

	select {
	case <-ctx.Done():
		return true, ctx.Err()
	case err := <-recvDone:
		if err != nil && err == ErrorPureRunnerNACK && isCachedReader {
			return false, err
		}
		return true, err
	case err := <-sendDone:
		return true, err
	}
}

func sendToRunner(protocolClient pb.RunnerProtocol_EngageClient, call pool.RunnerCall, modelJSON []byte, done chan error) {
	err := protocolClient.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{ModelsCallJson: string(modelJSON)}}})
	if err != nil {
		logrus.WithError(err).Error("Failed to send message to runner node")
		done <- err
		return
	}

	isEOF := false
	bodyReader := call.RequestBody()
	writeBuffer := make([]byte, MaxDataChunk)

	for !isEOF {
		// WARNING: blocking read.
		// IMPORTANT: make sure gin/agent actually times this out.
		n, err := bodyReader.Read(writeBuffer)

		if err != nil && err != io.EOF {
			logrus.WithError(err).Error("Failed to receive data from http client body")
		}

		// any IO error or n == 0 is an EOF for pure-runner
		isEOF = err != nil || n == 0
		data := writeBuffer[:n]

		sendErr := protocolClient.Send(&pb.ClientMsg{
			Body: &pb.ClientMsg_Data{
				Data: &pb.DataFrame{
					Data: data,
					Eof:  isEOF,
				},
			},
		})
		if sendErr != nil {
			logrus.WithError(sendErr).Error("Failed to send data frame with EOF to runner")
			done <- sendErr
			return
		}
	}
}

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, c pool.RunnerCall, done chan error) {
	w := c.ResponseWriter()
	defer close(done)

	for {
		msg, err := protocolClient.Recv()
		if err != nil {
			logrus.WithError(err).Error("Failed to receive message from runner")
			done <- err
			return
		}

		switch body := msg.Body.(type) {

		// Check for NACK from server if request was rejected.
		case *pb.RunnerMsg_Acknowledged:
			if !body.Acknowledged.Committed {
				logrus.Debugf("Runner didn't commit invocation request: %v", body.Acknowledged.Details)
				done <- ErrorPureRunnerNACK
				return
			}

		case *pb.RunnerMsg_ResultStart:
			switch meta := body.ResultStart.Meta.(type) {
			case *pb.CallResultStart_Http:
				for _, header := range meta.Http.Headers {
					w.Header().Set(header.Key, header.Value)
				}
			default:
				logrus.Errorf("Unhandled meta type in start message: %v", meta)
				// WARNING: we ignore this case, test/re-evaluate this
			}
		case *pb.RunnerMsg_Data:
			// WARNING: blocking write
			// IMPORTANT: make sure gin/agent times this out if blocked.
			w.Write(body.Data.Data)
		case *pb.RunnerMsg_Finished:
			if body.Finished.Success {
				logrus.Infof("Call finished successfully: %v", body.Finished.Details)
			} else {
				logrus.Infof("Call finished unsuccessfully: %v", body.Finished.Details)
			}
			return
		default:
			logrus.Errorf("Unhandled message type from runner: %v", body)
			// WARNING: we ignore this case, test/re-evaluate this
		}
	}
}
