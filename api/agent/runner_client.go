package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	pb "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/grpcutil"
	"github.com/sirupsen/logrus"
)

var (
	ErrorRunnerClosed    = errors.New("Runner is closed")
	ErrorPureRunnerNoEOF = errors.New("Purerunner missing EOF response")
)

const (
	// max buffer size for grpc data messages, 10K
	MaxDataChunk = 10 * 1024
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

func isRetriable(err error) bool {
	// A formal API error returned from pure-runner
	if models.GetAPIErrorCode(err) == models.GetAPIErrorCode(models.ErrCallTimeoutServerBusy) {
		return true
	}
	if err != nil {
		// engagement/recv errors could also be a 503.
		st := status.Convert(err)
		if int(st.Code()) == models.GetAPIErrorCode(models.ErrCallTimeoutServerBusy) {
			return true
		}
	}
	return false
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
		logrus.Infof("Engagement Context ended ctxErr=%v", ctx.Err())
		return true, ctx.Err()
	case recvErr := <-recvDone:
		return !isRetriable(recvErr), recvErr
	}
}

func sendToRunner(protocolClient pb.RunnerProtocol_EngageClient, call pool.RunnerCall) {
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

func parseError(details string) error {
	tokens := strings.SplitN(details, ":", 2)
	if len(tokens) != 2 || tokens[0] == "" || tokens[1] == "" {
		return errors.New(details)
	}
	code, err := strconv.ParseInt(tokens[0], 10, 64)
	if err != nil {
		return errors.New(details)
	}
	if code != 0 {
		return models.NewAPIError(int(code), errors.New(tokens[1]))
	}
	return errors.New(tokens[1])
}

func tryQueueError(err error, done chan error) {
	select {
	case done <- err:
	default:
	}
}

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, c pool.RunnerCall, done chan error) {
	w := c.ResponseWriter()
	defer close(done)

	isPartialWrite := false

DataLoop:
	for {
		msg, err := protocolClient.Recv()
		if err != nil {
			logrus.WithError(err).Info("Receive error from runner")
			tryQueueError(err, done)
			return
		}

		switch body := msg.Body.(type) {

		// Process HTTP header/status message. This may not arrive depending on
		// pure runners behavior. (Eg. timeout & no IO received from function)
		case *pb.RunnerMsg_ResultStart:
			switch meta := body.ResultStart.Meta.(type) {
			case *pb.CallResultStart_Http:
				logrus.Debugf("Received meta http result from runner Status=%v", meta.Http.StatusCode)
				for _, header := range meta.Http.Headers {
					w.Header().Set(header.Key, header.Value)
				}
				if meta.Http.StatusCode > 0 {
					w.WriteHeader(int(meta.Http.StatusCode))
				}
			default:
				logrus.Errorf("Unhandled meta type in start message: %v", meta)
			}

		// May arrive if function has output. We ignore EOF.
		case *pb.RunnerMsg_Data:
			logrus.Debugf("Received data from runner len=%d isEOF=%v", len(body.Data.Data), body.Data.Eof)
			if !isPartialWrite {
				// WARNING: blocking write
				n, err := w.Write(body.Data.Data)
				if n != len(body.Data.Data) {
					isPartialWrite = true
					logrus.WithError(err).Infof("Failed to write full response (%d of %d) to client", n, len(body.Data.Data))
					if err == nil {
						err = io.ErrShortWrite
					}
					tryQueueError(err, done)
				}
			}

		// Finish messages required for finish/finalize the processing.
		case *pb.RunnerMsg_Finished:
			logrus.Infof("Call finished Success=%v %v", body.Finished.Success, body.Finished.Details)
			if !body.Finished.Success {
				err := parseError(body.Finished.GetDetails())
				tryQueueError(err, done)
			}
			break DataLoop

		default:
			logrus.Error("Ignoring unknown message type %T from runner, possible client/server mismatch", body)
		}
	}

	// There should be an EOF following the last packet
	for {
		msg, err := protocolClient.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			logrus.WithError(err).Infof("Call Waiting EOF received error")
			tryQueueError(err, done)
			break
		}

		switch body := msg.Body.(type) {
		default:
			logrus.Infof("Call Waiting EOF ignoring message %T", body)
		}
		tryQueueError(ErrorPureRunnerNoEOF, done)
	}
}
