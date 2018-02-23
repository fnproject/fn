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
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/poolmanager"
	"github.com/sirupsen/logrus"
)

type NodePool interface {
	Runners(lbgID string) []Runner
	Shutdown()
}

type Runner interface {
	TryExec(ctx context.Context, call *call) (bool, error)
}

type gRPCNodePool struct {
	npm poolmanager.NodePoolManager
	mx  sync.RWMutex
	lbg map[string]*lbg // {lbgid -> *lbg}
	//TODO find a better place for this
	pki pkiData

	shutdown chan struct{}
}

// TODO need to go in a better place
type pkiData struct {
	ca   string
	key  string
	cert string
}

type lbg struct {
	mx      sync.RWMutex
	id      string
	runners map[string]*gRPCRunner
}

type gRPCRunner struct {
	// Need a WaitGroup of TryExec in flight
	wg      sync.WaitGroup
	address string
	conn    *grpc.ClientConn
	client  pb.RunnerProtocolClient
}

func NewgRPCNodePool(npmAddress string, cert string, key string, ca string, capacityAggregator poolmanager.CapacityAggregator) *gRPCNodePool {
	p := pkiData{
		ca:   ca,
		cert: cert,
		key:  key,
	}
	np := &gRPCNodePool{
		npm:      poolmanager.NewNodePoolManager(npmAddress, cert, key, ca),
		lbg:      make(map[string]*lbg),
		shutdown: make(chan struct{}),
		pki:      p,
	}
	go np.maintenance()
	// TODO do we need to persistent this ID in order to survive restart?
	lbID := id.New().String()

	np.npm.ScheduleUpdates(lbID, capacityAggregator, 1*time.Second)
	return np
}

func (np *gRPCNodePool) Runners(lbgID string) []Runner {
	np.mx.RLock()
	lbg, ok := np.lbg[lbgID]
	np.mx.RUnlock()

	if !ok {
		np.mx.Lock()
		lbg, ok = np.lbg[lbgID]
		if !ok {
			lbg = newLBG(lbgID)
			np.lbg[lbgID] = lbg
		}
		np.mx.Unlock()
	}

	return lbg.runnerList()
}

func (np *gRPCNodePool) Shutdown() {
}

func (np *gRPCNodePool) maintenance() {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-np.shutdown:
			return
		case <-ticker.C:
			// Reload any LBGroup information from NPM (pull for the moment, shift to listening to a stream later)
			np.reloadLBGmembership()
		}
	}

}

func newLBG(lbgId string) *lbg {
	return &lbg{
		id:      lbgId,
		runners: map[string]*gRPCRunner{},
	}
}

func (np *gRPCNodePool) reloadLBGmembership() {
	np.mx.RLock()
	defer np.mx.RUnlock() // XXX fix locking
	for lbgId, lbg := range np.lbg {
		lbg.reloadMembers(lbgId, np.npm, np.pki)
	}
}

func (lbg *lbg) runnerList() []Runner {
	lbg.mx.RLock()
	defer lbg.mx.RUnlock()
	runners := []Runner{}
	for _, r := range lbg.runners {
		runners = append(runners, r)
	}
	return runners
}

func (lbg *lbg) reloadMembers(lbgId string, npm poolmanager.NodePoolManager, p pkiData) {
	runners, err := npm.GetRunners(lbgId)
	if err != nil {
		// XXX log and fall out
	}
	lbg.mx.Lock()
	defer lbg.mx.Unlock()
	seen := map[string]bool{} // If we've seen a particular runner or not
	for _, addr := range runners {
		_, ok := lbg.runners[addr]
		if !ok {
			conn, client := runnerConnection(addr, lbgId, p)
			lbg.runners[addr] = &gRPCRunner{
				address: addr,
				conn:    conn,
				client:  client,
			}
		}
		seen[addr] = true
	}

	// Remove any runners that we have not encountered
	for addr, r := range lbg.runners {
		if _, ok := seen[addr]; !ok {
			r.close()
			delete(lbg.runners, addr)
		}
	}
}

func (r *gRPCRunner) close() {
	go func() {
		r.wg.Wait()
		r.conn.Close()
	}()
}

func runnerConnection(address, lbGroupID string, pki pkiData) (*grpc.ClientConn, pb.RunnerProtocolClient) {
	// Not connected, so create a connection with the TLS credentials
	//	logrus.WithField("lbg_id", lbGroupID).WithField("runner_addr", address).Info("Connecting to runner")
	ctx := context.Background()
	creds, err := createCredentials(pki.cert, pki.key, pki.ca)
	if err != nil {
		logrus.WithError(err).Error("Unable to create credentials to connect to runner node")

	}
	conn, err := blockingDial(ctx, address, creds)
	if err != nil {
		logrus.WithError(err).Error("Unable to connect to runner node")

	}

	// We don't explicitly close connections to runners. Instead, we won't reconnect to them
	// if they are shutdown and not active
	// defer conn.Close()

	protocolClient := pb.NewRunnerProtocolClient(conn)
	logrus.WithField("lbg_id", lbGroupID).WithField("runner_addr", address).Info("Connected to runner")

	return conn, protocolClient
}

func createCredentials(certPath string, keyPath string, caCertPath string) (credentials.TransportCredentials, error) {
	// Load the client certificates from disk
	certificate, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load client key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	ca, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}

	// Append the certificates from the CA
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.New("failed to append ca certs")
	}

	return credentials.NewTLS(&tls.Config{
		ServerName:   "127.0.0.1", // NOTE: this is required!
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
	}), nil
}

// the standard grpc dial does not block on connection failures and hence completely hides all TLS errors
func blockingDial(ctx context.Context, address string, creds credentials.TransportCredentials, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// grpc.Dial doesn't provide any information on permanent connection errors (like
	// TLS handshake failures). So in order to provide good error messages, we need a
	// custom dialer that can provide that info. That means we manage the TLS handshake.
	result := make(chan interface{}, 1)

	writeResult := func(res interface{}) {
		// non-blocking write: we only need the first result
		select {
		case result <- res:
		default:
		}
	}

	dialer := func(address string, timeout time.Duration) (net.Conn, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		conn, err := (&net.Dialer{Cancel: ctx.Done()}).Dial("tcp", address)
		if err != nil {
			writeResult(err)
			return nil, err
		}
		if creds != nil {
			conn, _, err = creds.ClientHandshake(ctx, address, conn)
			if err != nil {
				writeResult(err)
				return nil, err
			}
		}
		return conn, nil
	}

	// Even with grpc.FailOnNonTempDialError, this call will usually timeout in
	// the face of TLS handshake errors. So we can't rely on grpc.WithBlock() to
	// know when we're done. So we run it in a goroutine and then use result
	// channel to either get the channel or fail-fast.
	go func() {
		opts = append(opts,
			grpc.WithBlock(),
			grpc.FailOnNonTempDialError(true),
			grpc.WithDialer(dialer),
			grpc.WithInsecure(), // we are handling TLS, so tell grpc not to
		)
		conn, err := grpc.DialContext(ctx, address, opts...)
		var res interface{}
		if err != nil {
			res = err
		} else {
			res = conn
		}
		writeResult(res)
	}()

	select {
	case res := <-result:
		if conn, ok := res.(*grpc.ClientConn); ok {
			return conn, nil
		} else {
			return nil, res.(error)
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *gRPCRunner) TryExec(ctx context.Context, call *call) (bool, error) {
	r.wg.Add(1)
	defer r.wg.Done()

	// Move the submit trial here

	// Get app and route information
	// Construct model.Call with CONFIG in it already
	modelJSON, err := json.Marshal(call.Model())
	if err != nil {
		logrus.WithError(err).Error("Failed to encode model as JSON")
		// If we can't encode the model, no runner will ever be able to run this. Give up.
		return true, err
	}
	runnerConnection, err := r.client.Engage(context.Background())
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
			logrus.Errorf("Runner didn't commit invocation request: %v", body.Acknowledged.Details)
			return false, nil
			// Try the next runner
		} else {
			logrus.Info("Runner committed invocation request, sending data frames")
			done := make(chan struct{})
			go receiveFromRunner(runnerConnection, call, done)
			sendToRunner(call, runnerConnection)
			<-done
			return true, nil
		}
	default:
		logrus.Info("Unhandled message type received from runner: %v\n", msg)
		return true, nil
	}

}

func sendToRunner(call Call, protocolClient pb.RunnerProtocol_EngageClient) error {
	bodyReader, err := RequestReader(&call)
	if err != nil {
		logrus.WithError(err).Error("Unable to get reader for request body")
		return err
	}
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

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, call Call, done chan struct{}) {
	w, err := ResponseWriter(&call)

	if err != nil {
		logrus.WithError(err).Error("Unable to get response writer from call")
		return
	}

	for {
		msg, err := protocolClient.Recv()
		if err != nil {
			logrus.WithError(err).Error("Failed to receive message from runner")
			return
		}

		switch body := msg.Body.(type) {
		case *pb.RunnerMsg_ResultStart:
			switch meta := body.ResultStart.Meta.(type) {
			case *pb.CallResultStart_Http:
				for _, header := range meta.Http.Headers {
					(*w).Header().Set(header.Key, header.Value)
				}
			default:
				logrus.Errorf("Unhandled meta type in start message: %v", meta)
			}
		case *pb.RunnerMsg_Data:
			(*w).Write(body.Data.Data)
		case *pb.RunnerMsg_Finished:
			if body.Finished.Success {
				logrus.Infof("Call finished successfully: %v", body.Finished.Details)
			} else {
				logrus.Infof("Call finish unsuccessfully:: %v", body.Finished.Details)
			}
			close(done)
			return
		default:
			logrus.Errorf("Unhandled message type from runner: %v", body)
		}
	}
}
