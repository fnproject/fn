package agent

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/grpcutil"

	pb_empty "github.com/golang/protobuf/ptypes/empty"
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

func SecureGRPCRunnerFactory(addr string, tlsConf *tls.Config) (pool.Runner, error) {
	conn, client, err := runnerConnection(addr, tlsConf)
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

// implements Runner
func (r *gRPCRunner) Close(context.Context) error {
	r.shutWg.CloseGroup()
	return r.conn.Close()
}

func runnerConnection(address string, tlsConf *tls.Config) (*grpc.ClientConn, pb.RunnerProtocolClient, error) {

	ctx := context.Background()
	logger := common.Logger(ctx).WithField("runner_addr", address)
	ctx = common.WithLogger(ctx, logger)

	var creds credentials.TransportCredentials
	if tlsConf != nil {
		creds = credentials.NewTLS(tlsConf)
	}

	// we want to set a very short timeout to fail-fast if something goes wrong
	conn, err := grpcutil.DialWithBackoff(ctx, address, creds, 100*time.Millisecond, grpc.DefaultBackoffConfig)
	if err != nil {
		logger.WithError(err).Error("Unable to connect to runner node")
	}

	protocolClient := pb.NewRunnerProtocolClient(conn)
	logger.Info("Connected to runner")

	return conn, protocolClient, nil
}

// implements Runner
func (r *gRPCRunner) Address() string {
	return r.address
}

func isTooBusy(err error) bool {
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

// Translate runner.RunnerStatus to runnerpool.RunnerStatus
func TranslateGRPCStatusToRunnerStatus(status *pb.RunnerStatus) *pool.RunnerStatus {
	if status == nil {
		return nil
	}

	creat, _ := common.ParseDateTime(status.CreatedAt)
	start, _ := common.ParseDateTime(status.StartedAt)
	compl, _ := common.ParseDateTime(status.CompletedAt)

	return &pool.RunnerStatus{
		ActiveRequestCount: status.Active,
		RequestsReceived:   status.RequestsReceived,
		RequestsHandled:    status.RequestsHandled,
		StatusFailed:       status.Failed,
		Cached:             status.Cached,
		StatusId:           status.Id,
		Details:            status.Details,
		ErrorCode:          status.ErrorCode,
		ErrorStr:           status.ErrorStr,
		CreatedAt:          creat,
		StartedAt:          start,
		CompletedAt:        compl,
	}
}

// implements Runner
func (r *gRPCRunner) Status(ctx context.Context) (*pool.RunnerStatus, error) {
	log := common.Logger(ctx).WithField("runner_addr", r.address)
	rid := common.RequestIDFromContext(ctx)
	if rid != "" {
		// Create a new gRPC metadata where we store the request ID
		mp := metadata.Pairs(common.RequestIDContextKey, rid)
		ctx = metadata.NewOutgoingContext(ctx, mp)
	}

	status, err := r.client.Status(ctx, &pb_empty.Empty{})
	log.WithError(err).Debugf("Status Call %+v", status)
	return TranslateGRPCStatusToRunnerStatus(status), err
}

// implements Runner
func (r *gRPCRunner) TryExec(ctx context.Context, call pool.RunnerCall) (bool, error) {
	log := common.Logger(ctx).WithField("runner_addr", r.address)

	log.Debug("Attempting to place call")
	if !r.shutWg.AddSession(1) {
		// try another runner if this one is closed.
		return false, ErrorRunnerClosed
	}
	defer r.shutWg.DoneSession()

	// extract the call's model data to pass on to the pure runner
	modelJSON, err := json.Marshal(call.Model())
	if err != nil {
		log.WithError(err).Error("Failed to encode model as JSON")
		// If we can't encode the model, no runner will ever be able to run this. Give up.
		return true, err
	}

	rid := common.RequestIDFromContext(ctx)
	if rid != "" {
		// Create a new gRPC metadata where we store the request ID
		mp := metadata.Pairs(common.RequestIDContextKey, rid)
		ctx = metadata.NewOutgoingContext(ctx, mp)
	}
	runnerConnection, err := r.client.Engage(ctx)
	if err != nil {
		log.WithError(err).Error("Unable to create client to runner node")
		// Try on next runner
		return false, err
	}

	err = runnerConnection.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{
		ModelsCallJson: string(modelJSON),
		SlotHashId:     hex.EncodeToString([]byte(call.SlotHashId())),
		Extensions:     call.Extensions(),
	}}})
	if err != nil {
		log.WithError(err).Error("Failed to send message to runner node")
		// Try on next runner
		return false, err
	}

	// After this point TryCall was sent, we assume "COMMITTED" unless pure runner
	// send explicit NACK

	recvDone := make(chan error, 1)

	go receiveFromRunner(ctx, runnerConnection, r.address, call, recvDone)
	go sendToRunner(ctx, runnerConnection, r.address, call)

	select {
	case <-ctx.Done():
		log.Infof("Engagement Context ended ctxErr=%v", ctx.Err())
		return true, ctx.Err()
	case recvErr := <-recvDone:
		if isTooBusy(recvErr) {
			// Try on next runner
			return false, models.ErrCallTimeoutServerBusy
		}
		return true, recvErr
	}
}

func sendToRunner(ctx context.Context, protocolClient pb.RunnerProtocol_EngageClient, runnerAddress string, call pool.RunnerCall) {
	bodyReader := call.RequestBody()
	writeBuffer := make([]byte, MaxDataChunk)

	log := common.Logger(ctx).WithField("runner_addr", runnerAddress)
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
			log.WithError(err).Error("Failed to receive data from http client body")
		}

		// any IO error or n == 0 is an EOF for pure-runner
		isEOF := err != nil || n == 0
		data := writeBuffer[:n]

		log.Debugf("Sending %d bytes of data isEOF=%v to runner", n, isEOF)
		sendErr := protocolClient.Send(&pb.ClientMsg{
			Body: &pb.ClientMsg_Data{
				Data: &pb.DataFrame{
					Data: data,
					Eof:  isEOF,
				},
			},
		})
		if sendErr != nil {
			// It's often normal to receive an EOF here as we optimistically start sending body until a NACK
			// from the runner. Let's ignore EOF and rely on recv side to catch premature EOF.
			if sendErr != io.EOF {
				log.WithError(sendErr).Errorf("Failed to send data frame size=%d isEOF=%v", n, isEOF)
			}
			return
		}
		if isEOF {
			return
		}
	}
}

func parseError(msg *pb.CallFinished) error {
	if msg.GetSuccess() {
		return nil
	}
	eCode := msg.GetErrorCode()
	eStr := msg.GetErrorStr()
	if eStr == "" {
		eStr = "Unknown Error From Pure Runner"
	}
	return models.NewAPIError(int(eCode), errors.New(eStr))
}

func tryQueueError(err error, done chan error) {
	select {
	case done <- err:
	default:
	}
}

func translateDate(dt string) time.Time {
	if dt != "" {
		trx, err := common.ParseDateTime(dt)
		if err == nil {
			return time.Time(trx)
		}
	}
	return time.Time{}
}

func recordFinishStats(ctx context.Context, msg *pb.CallFinished) {

	creatTs := translateDate(msg.GetCreatedAt())
	startTs := translateDate(msg.GetStartedAt())
	complTs := translateDate(msg.GetCompletedAt())

	// Validate this as info *is* coming from runner and its local clock.
	if !creatTs.IsZero() && !startTs.IsZero() && !complTs.IsZero() && !startTs.Before(creatTs) && !complTs.Before(startTs) {
		statsLBAgentRunnerSchedLatency(ctx, startTs.Sub(creatTs))
		statsLBAgentRunnerExecLatency(ctx, complTs.Sub(startTs))
	}
}

func receiveFromRunner(ctx context.Context, protocolClient pb.RunnerProtocol_EngageClient, runnerAddress string, c pool.RunnerCall, done chan error) {
	w := c.ResponseWriter()
	defer close(done)

	log := common.Logger(ctx).WithField("runner_addr", runnerAddress)
	isPartialWrite := false

DataLoop:
	for {
		msg, err := protocolClient.Recv()
		if err != nil {
			log.WithError(err).Info("Receive error from runner")
			tryQueueError(err, done)
			return
		}

		switch body := msg.Body.(type) {

		// Process HTTP header/status message. This may not arrive depending on
		// pure runners behavior. (Eg. timeout & no IO received from function)
		case *pb.RunnerMsg_ResultStart:
			switch meta := body.ResultStart.Meta.(type) {
			case *pb.CallResultStart_Http:
				log.Debugf("Received meta http result from runner Status=%v", meta.Http.StatusCode)
				for _, header := range meta.Http.Headers {
					w.Header().Set(header.Key, header.Value)
				}
				if meta.Http.StatusCode > 0 {
					w.WriteHeader(int(meta.Http.StatusCode))
				}
			default:
				log.Errorf("Unhandled meta type in start message: %v", meta)
			}

		// May arrive if function has output. We ignore EOF.
		case *pb.RunnerMsg_Data:
			log.Debugf("Received data from runner len=%d isEOF=%v", len(body.Data.Data), body.Data.Eof)
			if !isPartialWrite {
				// WARNING: blocking write
				n, err := w.Write(body.Data.Data)
				if n != len(body.Data.Data) {
					isPartialWrite = true
					log.WithError(err).Infof("Failed to write full response (%d of %d) to client", n, len(body.Data.Data))
					if err == nil {
						err = io.ErrShortWrite
					}
					tryQueueError(err, done)
				}
			}

		// Finish messages required for finish/finalize the processing.
		case *pb.RunnerMsg_Finished:
			log.Infof("Call finished Success=%v %v", body.Finished.Success, body.Finished.Details)
			recordFinishStats(ctx, body.Finished)
			if !body.Finished.Success {
				err := parseError(body.Finished)
				tryQueueError(err, done)
			}
			break DataLoop

		default:
			log.Errorf("Ignoring unknown message type %T from runner, possible client/server mismatch", body)
		}
	}

	// There should be an EOF following the last packet
	for {
		msg, err := protocolClient.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.WithError(err).Infof("Call Waiting EOF received error")
			tryQueueError(err, done)
			break
		}

		switch body := msg.Body.(type) {
		default:
			log.Infof("Call Waiting EOF ignoring message %T", body)
		}
		tryQueueError(ErrorPureRunnerNoEOF, done)
	}
}

var _ pool.Runner = &gRPCRunner{}
