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
	ErrorRunnerClosed    = errors.New("Runner is closed")
	ErrorPureRunnerNACK  = errors.New("Purerunner NACK response")
	ErrorPureRunnerNoEOF = errors.New("Purerunner missing EOF response")
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
		// try another runner if this one is closed.
		return false, ErrorRunnerClosed
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
	err = runnerConnection.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{ModelsCallJson: string(modelJSON)}}})
	if err != nil {
		logrus.WithError(err).Error("Failed to send message to runner node")
		return true, err
	}

	recvDone := make(chan error, 1)

	go receiveFromRunner(runnerConnection, call, recvDone)
	go sendToRunner(runnerConnection, call)

	select {
	case <-ctx.Done():
		logrus.Infof("Engagement ended ctxErr=%v", ctx.Err())
		return true, ctx.Err()
	case recvErr := <-recvDone:
		if recvErr != nil {
			logrus.Infof("Engagement ended with recvErr=%v", recvErr)
		}
		return recvErr != ErrorPureRunnerNACK, recvErr
	}
}

func sendToRunner(protocolClient pb.RunnerProtocol_EngageClient, call pool.RunnerCall) {
	total := 0
	bodyReader := call.RequestBody()
	writeBuffer := make([]byte, MaxDataChunk)

	// IMPORTANT: IO Read below can fail in multiple go-routine cases (in retry
	// case especially if receiveFromRunner go-routine receives a NACK while sendToRunner is
	// already blocked on a read) or in the case of reading the http body multiple times (retries.)
	// Normally http.Request.Body can be read once. However runner_client users should implement/add
	// http.Request.GetBody() function and cache the body content in the request.
	// See lb_agent setRequestGetBody() which handles this. With GetBody installed,
	// the 'Read' below is an actually non-blocking operation since GetBody() should hand out
	// a new instance of io.ReadCloser() that allows repetitive reads on the http body.
	for {
		// WARNING: blocking read.
		n, err := bodyReader.Read(writeBuffer)
		if err != nil && err != io.EOF {
			logrus.WithError(err).Error("Failed to receive data from http client body")
		}

		// any IO error or n == 0 is an EOF for pure-runner
		isEOF := err != nil || n == 0
		data := writeBuffer[:n]
		total += n

		logrus.Debugf("Sending %d bytes of data isEOF=%v to runner", n, isEOF)
		sendErr := protocolClient.Send(&pb.ClientMsg{
			Body: &pb.ClientMsg_Data{
				Data: &pb.DataFrame{
					Data: data,
					Eof:  isEOF,
				},
			},
		})
		if sendErr != nil {
			logrus.WithError(sendErr).Errorf("Failed to send data frame size=%d isEOF=%v", n, isEOF)
			return
		}
		if isEOF {
			return
		}
	}
}

func tryQueueError(err error, done chan error) {
	logrus.WithError(err).Debug("Detected receive side error")
	select {
	case done <- err:
	default:
	}
}

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, c pool.RunnerCall, done chan error) {
	w := c.ResponseWriter()
	defer close(done)

DataLoop:
	for {
		msg, err := protocolClient.Recv()
		if err != nil {
			tryQueueError(err, done)
			return
		}

		switch body := msg.Body.(type) {

		// Check for NACK from server if request was rejected.
		case *pb.RunnerMsg_Acknowledged:
			if !body.Acknowledged.Committed {
				logrus.Debugf("Runner didn't commit invocation request: %v", body.Acknowledged.Details)
				tryQueueError(ErrorPureRunnerNACK, done)
				break DataLoop
			} else {
				// WARNING: we ignore this case, test/re-evaluate this
				logrus.WithError(err).Error("ACK received from runner, possible client/server mismatch")
			}

		case *pb.RunnerMsg_ResultStart:
			switch meta := body.ResultStart.Meta.(type) {
			case *pb.CallResultStart_Http:
				for _, header := range meta.Http.Headers {
					w.Header().Set(header.Key, header.Value)
				}
				if meta.Http.StatusCode > 0 {
					w.WriteHeader(int(meta.Http.StatusCode))
				}
			default:
				// WARNING: we ignore this case, test/re-evaluate this
				logrus.Errorf("Unhandled meta type in start message: %v", meta)
			}
		case *pb.RunnerMsg_Data:
			logrus.Debugf("Received data from runner len=%d isEOF=%v", len(body.Data.Data), body.Data.Eof)

			// WARNING: blocking write
			n, err := w.Write(body.Data.Data)
			if n != len(body.Data.Data) {
				if err == nil {
					err = io.ErrShortWrite
				}
				tryQueueError(err, done)
				break DataLoop
			}

		case *pb.RunnerMsg_Finished:
			logrus.Infof("Call finished Success=%v %v", body.Finished.Success, body.Finished.Details)
			break DataLoop

		default:
			logrus.Errorf("Unhandled message type from runner: %v", body)
			// WARNING: we ignore this case, test/re-evaluate this
		}
	}

	// There should be an EOF following the last packet
	for {
		_, err := protocolClient.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			tryQueueError(err, done)
			break
		}

		tryQueueError(ErrorPureRunnerNoEOF, done)
	}
}
