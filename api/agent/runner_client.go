package agent

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/fnproject/fn/api/agent/grpc"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/grpcutil"
	"github.com/sirupsen/logrus"
)

type gRPCRunner struct {
	// Need a WaitGroup of TryExec in flight
	wg      sync.WaitGroup
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
		address: addr,
		conn:    conn,
		client:  client,
	}, nil
}

// Close waits until the context is closed for all inflight requests
// to complete prior to terminating the underlying grpc connection
func (r *gRPCRunner) Close(ctx context.Context) error {
	err := make(chan error, 1)
	go func() {
		defer close(err)
		r.wg.Wait()
		err <- r.conn.Close()
	}()

	select {
	case e := <-err:
		return e
	case <-ctx.Done():
		return ctx.Err() // context timed out while waiting
	}
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
	r.wg.Add(1)
	defer r.wg.Done()

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

	err = runnerConnection.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{ModelsCallJson: string(modelJSON)}}})
	if err != nil {
		logrus.WithError(err).Error("Failed to send message to runner node")
		return false, err
	}
	msg, err := runnerConnection.Recv()
	if err != nil {
		logrus.WithError(err).Error("Failed to receive first message from runner node")
		return false, err
	}

	switch body := msg.Body.(type) {
	case *pb.RunnerMsg_Acknowledged:
		if !body.Acknowledged.Committed {
			logrus.Debugf("Runner didn't commit invocation request: %v", body.Acknowledged.Details)
			return false, nil
			// Try the next runner
		}
		logrus.Debug("Runner committed invocation request, sending data frames")
		done := make(chan error)
		go receiveFromRunner(runnerConnection, call, done)
		sendToRunner(call, runnerConnection)
		return true, <-done

	default:
		logrus.Errorf("Unhandled message type received from runner: %v\n", msg)
		return true, nil
	}

}

func sendToRunner(call pool.RunnerCall, protocolClient pb.RunnerProtocol_EngageClient) error {
	bodyReader := call.RequestBody()
	writeBufferSize := 10 * 1024 // 10KB
	writeBuffer := make([]byte, writeBufferSize)
	for {
		n, err := bodyReader.Read(writeBuffer)
		logrus.Debugf("Wrote %v bytes to the runner", n)

		if err == io.EOF {
			err = protocolClient.Send(&pb.ClientMsg{
				Body: &pb.ClientMsg_Data{
					Data: &pb.DataFrame{
						Data: writeBuffer,
						Eof:  true,
					},
				},
			})
			if err != nil {
				logrus.WithError(err).Error("Failed to send data frame with EOF to runner")
			}
			break
		}
		err = protocolClient.Send(&pb.ClientMsg{
			Body: &pb.ClientMsg_Data{
				Data: &pb.DataFrame{
					Data: writeBuffer,
					Eof:  false,
				},
			},
		})
		if err != nil {
			logrus.WithError(err).Error("Failed to send data frame")
			return err
		}
	}
	return nil
}

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, c pool.RunnerCall, done chan error) {
	w := c.ResponseWriter()

	for {
		msg, err := protocolClient.Recv()
		if err != nil {
			logrus.WithError(err).Error("Failed to receive message from runner")
			done <- err
			return
		}

		switch body := msg.Body.(type) {
		case *pb.RunnerMsg_ResultStart:
			switch meta := body.ResultStart.Meta.(type) {
			case *pb.CallResultStart_Http:
				for _, header := range meta.Http.Headers {
					w.Header().Set(header.Key, header.Value)
				}
			default:
				logrus.Errorf("Unhandled meta type in start message: %v", meta)
			}
		case *pb.RunnerMsg_Data:
			w.Write(body.Data.Data)
		case *pb.RunnerMsg_Finished:
			if body.Finished.Success {
				logrus.Infof("Call finished successfully: %v", body.Finished.Details)
			} else {
				logrus.Infof("Call finished unsuccessfully: %v", body.Finished.Details)
			}
			// There should be an EOF following the last packet
			if _, err := protocolClient.Recv(); err != io.EOF {
				logrus.WithError(err).Error("Did not receive expected EOF from runner stream")
				done <- err
			}
			close(done)
			return
		default:
			logrus.Errorf("Unhandled message type from runner: %v", body)
		}
	}
}
