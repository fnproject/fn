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
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
)

type lbAgent struct {
	runnerAddress  string
	delegatedAgent agent.Agent
	cert           string
	key            string
	ca             string
}

func New(runnerAddress string, agent agent.Agent, cert string, key string, ca string) agent.Agent {
	return &lbAgent{
		runnerAddress:  runnerAddress,
		delegatedAgent: agent,
		cert:           cert,
		key:            key,
		ca:             ca,
	}
}

// GetCall delegates to the wrapped agent
func (a *lbAgent) GetCall(opts ...agent.CallOpt) (agent.Call, error) {
	return a.delegatedAgent.GetCall(opts...)
}

func (a *lbAgent) Close() error {
	return nil
}

func (a *lbAgent) Submit(call agent.Call) error {

	// Get app and route information
	// Construct model.Call with CONFIG in it already
	// Is there a runner available for the lbgroup?
	// If not, then ask for capacity
	// If there is, call the runner over gRPC with the Call object

	// Runner URL won't be a config option here, but will be obtained from
	// the node pool manager

	// Create a connection with the TLS credentials
	ctx := context.Background()
	creds, err := createCredentials(a.cert, a.key, a.ca)
	if err != nil {
		logrus.WithError(err).Error("Unable to create credentials to connect to runner node")
		return err
	}
	conn, err := blockingDial(ctx, a.runnerAddress, creds)
	if err != nil {
		logrus.WithError(err).Error("Unable to connect to runner node")
		return fmt.Errorf("could not dial %s: %s", a.runnerAddress, err)
	}

	defer conn.Close()

	c := pb.NewRunnerProtocolClient(conn)

	protocolClient, err := c.Engage(context.Background())
	if err != nil {
		logrus.WithError(err).Error("Unable to create client to runner node")
		return err
	}

	modelJSON, err := json.Marshal(call.Model())
	if err != nil {
		logrus.WithError(err).Error("Failed to encode model as JSON")
		return err
	}

	err = protocolClient.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{ModelsCallJson: string(modelJSON)}}})
	msg, err := protocolClient.Recv()

	if err != nil {
		logrus.WithError(err).Error("Failed to send message to runner node")
		return err
	}

	switch body := msg.Body.(type) {
	case *pb.RunnerMsg_Acknowledged:
		if !body.Acknowledged.Committed {
			logrus.Errorf("Runner didn't commit invocation request: %v", body.Acknowledged.Details)
		} else {
			logrus.Info("Runner committed invocation request, sending data frames")

		}
	default:
		logrus.Info("Unhandled message type received from runner: %v\n", msg)
	}

	return nil
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
