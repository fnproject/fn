package lb

// This is the agent impl for LB nodes

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
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/fnproject/fn/api/agent"
	pb "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/fnproject/fn/poolmanager"
)

const (
	runnerReconnectInterval = 5 * time.Second
	// sleep time to attempt placement across all runners before retrying
	retryWaitInterval = 10 * time.Millisecond
	// sleep time when scaling from 0 to 1 runners
	noCapacityWaitInterval = 1 * time.Second
	// amount of time to wait to place a request on a runner
	placementTimeout = 15 * time.Second
)

type lbAgent struct {
	delegatedAgent     agent.Agent
	capacityAggregator poolmanager.CapacityAggregator
	npm                poolmanager.NodePoolManager
	cert               string
	key                string
	ca                 string
	runnerAddresses    *sync.Map
	connections        *sync.Map
}

type syncedSlice struct {
	mtx    *sync.RWMutex
	values []string
}

func newSyncedSlice() *syncedSlice {
	return &syncedSlice{mtx: &sync.RWMutex{}}
}

// returns a thread-safe copy of the original slice
func (ss *syncedSlice) load() []string {
	ss.mtx.RLock()
	defer ss.mtx.RUnlock()

	addrs := make([]string, len(ss.values))
	copy(addrs, ss.values)
	return addrs
}

func (ss *syncedSlice) store(values []string) {
	ss.mtx.Lock()
	defer ss.mtx.Unlock()

	ss.values = make([]string, len(values))
	copy(ss.values, values)
}

func New(npmAddress string, agent agent.Agent, cert string, key string, ca string) agent.Agent {

	a := &lbAgent{
		runnerAddresses:    &sync.Map{}, //make(map[string][]string)
		delegatedAgent:     agent,
		capacityAggregator: poolmanager.NewCapacityAggregator(),
		npm:                poolmanager.NewNodePoolManager(npmAddress, cert, key, ca),
		cert:               cert,
		key:                key,
		ca:                 ca,
		connections:        &sync.Map{}, //map[string]map[string](pb.RunnerProtocol_EngageClient))
	}

	go a.maintainConnectionToRunners()
	// TODO do we need to persistent this ID in order to survive restart?
	lbID := id.New().String()
	a.npm.ScheduleUpdates(lbID, a.capacityAggregator, 1*time.Second)
	return a
}

func (a *lbAgent) connectToRunner(lbGroupID string, address string, addresses *sync.Map) {
	// Not connected, so create a connection with the TLS credentials
	logrus.WithField("lbg_id", lbGroupID).WithField("runner_addr", address).Info("Connecting to runner")
	ctx := context.Background()
	creds, err := createCredentials(a.cert, a.key, a.ca)
	if err != nil {
		logrus.WithError(err).Error("Unable to create credentials to connect to runner node")
		return
	}
	conn, err := blockingDial(ctx, address, creds)
	if err != nil {
		logrus.WithError(err).Error("Unable to connect to runner node")
		return
	}

	// We don't explicitly close connections to runners. Instead, we won't reconnect to them
	// if they are shutdown and not active
	// defer conn.Close()

	protocolClient := pb.NewRunnerProtocolClient(conn)
	logrus.WithField("lbg_id", lbGroupID).WithField("runner_addr", address).Info("Connected to runner")
	addresses.Store(address, protocolClient)
}

func (a *lbAgent) refreshGroupConnections(lbGroupId string, runnerAddrs []string) {
	// clean up any connections that are no longer advertised
	c, ok := a.connections.Load(lbGroupId)
	if ok {
		conns := c.(*sync.Map)
		conns.Range(func(k, v interface{}) bool {
			addr := k.(string)
			found := false
			for _, address := range runnerAddrs {
				if address == addr {
					found = true
					break
				}
			}

			if !found {
				logrus.WithField("lbg_id", lbGroupId).WithField("runner_address", addr).Debug("Removing drained connection")
				conns.Delete(addr)
				// TODO expose a way of closing the grpc connection
				// v.(pb.RunnerProtocolClient).Close()
			}
			return true
		})

	}

	for _, address := range runnerAddrs {
		c, ok := a.connections.Load(lbGroupId)
		if !ok {
			c = &sync.Map{}
			a.connections.Store(lbGroupId, c)
		}
		conns, ok := c.(*sync.Map)
		if !ok {
			logrus.Warn("Found wrong type in connections map!")
			continue
		}

		// create conn
		if _, connected := conns.Load(address); !connected {
			a.connectToRunner(lbGroupId, address, conns)
		}
	}
}

func (a *lbAgent) maintainConnectionToRunners() {
	for {
		a.runnerAddresses.Range(func(k, v interface{}) bool {
			lbGroupId := k.(string)
			runnerAddrs := v.(*syncedSlice)
			a.refreshGroupConnections(lbGroupId, runnerAddrs.load())
			return true
		})
		time.Sleep(runnerReconnectInterval)
	}
}

// GetCall delegates to the wrapped agent
func (a *lbAgent) GetCall(opts ...agent.CallOpt) (agent.Call, error) {
	return a.delegatedAgent.GetCall(opts...)
}

func (a *lbAgent) Close() error {
	a.npm.Shutdown()
	return nil
}

func GetGroupID(call *models.Call) string {
	// TODO we need to make LBGroups part of data model so at the moment we just fake it
	// with this dumb method
	return "foobar"
}

func (a *lbAgent) Submit(call agent.Call) error {
	memMb := call.Model().Memory
	lbGroupID := GetGroupID(call.Model())

	capacityRequest := &poolmanager.CapacityEntry{TotalMemoryMb: memMb}
	a.capacityAggregator.AssignCapacity(capacityRequest, lbGroupID)
	// TODO verify that when we leave this method the call is in a completed or failed state
	// so it is safe to remove capacity
	defer a.capacityAggregator.ReleaseCapacity(capacityRequest, lbGroupID)

	deadline := time.Now().Add(placementTimeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("Unable to invoke function, no runner nodes accepted request")
		}

		// TODO we might need to diff new runner set with previous and explicitly close dropped ones
		runnerList, err := a.npm.GetRunners(lbGroupID)
		if err != nil {
			logrus.WithError(err).Info("Failed to get runners from node pool manager")

		} else {
			logrus.WithField("runners", len(runnerList)).Info("Updating runner list")

			if runners, ok := a.runnerAddresses.Load(lbGroupID); ok {
				runners.(*syncedSlice).store(runnerList)
			} else {
				runners := newSyncedSlice()
				runners.store(runnerList)
				a.runnerAddresses.Store(lbGroupID, runners)
			}

			a.refreshGroupConnections(lbGroupID, runnerList)
		}

		rmap, ok := a.connections.Load(lbGroupID)
		if !ok {
			logrus.WithField("lbg_id", lbGroupID).Debug("No runner nodes available")
			time.Sleep(noCapacityWaitInterval)
			continue
		}

		runnerMap, ok := rmap.(*sync.Map)
		if !ok {
			logrus.Warn("Runner map is the wrong type")
			return fmt.Errorf("Unable to invoke function, no runner nodes accepted request")
		}

		processedRequest := false
		var processingError error

		runnerMap.Range(func(k, v interface{}) bool {
			//address, protocolClient
			address := k.(string)
			protocolClient := v.(pb.RunnerProtocolClient)

			// Get app and route information
			// Construct model.Call with CONFIG in it already
			modelJSON, err := json.Marshal(call.Model())
			if err != nil {
				logrus.WithError(err).Error("Failed to encode model as JSON")
				processingError = err
				processedRequest = true
				return false
			}

			runnerConnection, err := protocolClient.Engage(context.Background())
			if err != nil {
				logrus.WithError(err).Error("Unable to create client to runner node")
				return false
			}

			err = runnerConnection.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{ModelsCallJson: string(modelJSON)}}})
			msg, err := runnerConnection.Recv()

			if err != nil {
				logrus.WithError(err).Error("Failed to send message to runner node")
				// Should probably remove the runner node from the list of connections
				a.connections.Delete(address)
				// assume connection was dropped, try on next runner
				return true
			}

			switch body := msg.Body.(type) {
			case *pb.RunnerMsg_Acknowledged:
				if !body.Acknowledged.Committed {
					logrus.Errorf("Runner didn't commit invocation request: %v", body.Acknowledged.Details)
					// Try the next runner
				} else {
					logrus.Info("Runner committed invocation request, sending data frames")
					done := make(chan struct{})
					go receiveFromRunner(runnerConnection, call, done)
					_ = sendToRunner(call, runnerConnection)
					<-done
					processedRequest = true
					return false
				}
			default:
				logrus.Info("Unhandled message type received from runner: %v\n", msg)
			}

			return true
		})

		if processedRequest {
			if processingError != nil {
				return processingError
			}
			return nil
		}

		time.Sleep(retryWaitInterval)
	}
}

func (a *lbAgent) Stats() agent.Stats {
	return agent.Stats{
		Queue:    0,
		Running:  0,
		Complete: 0,
		Failed:   0,
		Apps:     make(map[string]agent.AppStats),
	}
}

func (a *lbAgent) PromHandler() http.Handler {
	return nil
}

func (a *lbAgent) AddCallListener(fnext.CallListener) {

}

func (a *lbAgent) Enqueue(context.Context, *models.Call) error {
	logrus.Fatal("Enqueue not implemented. Panicking.")
	return nil
}

func sendToRunner(call agent.Call, protocolClient pb.RunnerProtocol_EngageClient) error {
	bodyReader, err := agent.RequestReader(&call)
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

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, call agent.Call, done chan struct{}) {
	w, err := agent.ResponseWriter(&call)

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
