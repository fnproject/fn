package lb

// This is the agent impl for LB nodes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
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
)

type lbAgent struct {
	delegatedAgent     agent.Agent
	capacityAggregator poolmanager.CapacityAggregator
	cert               string
	key                string
	ca                 string
	runnerAddresses    []string
	connections        map[string](pb.RunnerProtocol_EngageClient)
}

func New(runnerAddress string, agent agent.Agent, cert string, key string, ca string) agent.Agent {

	var addresses []string
	addresses = append(addresses, runnerAddress)

	a := &lbAgent{
		runnerAddresses:    addresses,
		delegatedAgent:     agent,
		capacityAggregator: poolmanager.NewCapacityAggregator(),
		cert:               cert,
		key:                key,
		ca:                 ca,
		connections:        make(map[string](pb.RunnerProtocol_EngageClient)),
	}

	go maintainConnectionToRunners(a)
	// TODO do we need to persistent this ID in order to survive restart?
	lbID := id.New().String()
	poolmanager.CapacityUpdatesSchedule("localhost:8888", lbID, a.capacityAggregator, 1*time.Second)
	return a
}

func maintainConnectionToRunners(a *lbAgent) {
	for {
		// Given the list of runner addresses, see if there is a connection in the connection map
		for _, address := range a.runnerAddresses {
			if _, connected := a.connections[address]; !connected {
				// Not connected, so create a connection with the TLS credentials
				logrus.Infof("Connecting to runner %v", address)
				ctx := context.Background()
				creds, err := createCredentials(a.cert, a.key, a.ca)
				if err != nil {
					logrus.WithError(err).Error("Unable to create credentials to connect to runner node")
					continue
				}
				conn, err := blockingDial(ctx, address, creds)
				if err != nil {
					logrus.WithError(err).Error("Unable to connect to runner node")

					continue
				}

				defer conn.Close()

				c := pb.NewRunnerProtocolClient(conn)
				protocolClient, err := c.Engage(context.Background())
				if err != nil {
					logrus.WithError(err).Error("Unable to create client to runner node")
					continue
				}

				a.connections[address] = protocolClient
			}
		}
		time.Sleep(runnerReconnectInterval)
	}
}

// GetCall delegates to the wrapped agent
func (a *lbAgent) GetCall(opts ...agent.CallOpt) (agent.Call, error) {
	return a.delegatedAgent.GetCall(opts...)
}

func (a *lbAgent) Close() error {
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

	// TODO we need to sleep and refresh list of runners to give new capacity a chance to show
	if len(a.connections) <= 0 {
		logrus.Error("No runner nodes available")
		return fmt.Errorf("Unable to invoke function, no runner nodes are available")
	}

	// Work through the connected runner nodes, submitting the request to each
	for address, protocolClient := range a.connections {
		// Get app and route information
		// Construct model.Call with CONFIG in it already
		modelJSON, err := json.Marshal(call.Model())
		if err != nil {
			logrus.WithError(err).Error("Failed to encode model as JSON")
			return err
		}

		err = protocolClient.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{ModelsCallJson: string(modelJSON)}}})
		msg, err := protocolClient.Recv()

		if err != nil {
			logrus.WithError(err).Error("Failed to send message to runner node")
			// Should probably remove the runner node from the list of connections
			delete(a.connections, address)
			return err
		}

		switch body := msg.Body.(type) {
		case *pb.RunnerMsg_Acknowledged:
			if !body.Acknowledged.Committed {
				logrus.Errorf("Runner didn't commit invocation request: %v", body.Acknowledged.Details)
				// Try the next runner
			} else {
				logrus.Info("Runner committed invocation request, sending data frames")
				return nil

			}
		default:
			logrus.Info("Unhandled message type received from runner: %v\n", msg)
		}
	}

	// Ask for some capacity!

	return fmt.Errorf("Unable to invoke function, no runner nodes accepted request")
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
