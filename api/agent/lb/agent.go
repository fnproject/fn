package lb

// This is the agent impl for LB nodes

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/fnproject/fn/api/agent"
	pb "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
)

type lbAgent struct {
	runnerAddress  string
	delegatedAgent agent.Agent
}

func New(runnerAddress string, agent agent.Agent, cert string, key string, ca string) agent.Agent {
	return &lbAgent{
		runnerAddress:  runnerAddress,
		delegatedAgent: agent,
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

	conn, err := grpc.Dial(a.runnerAddress, grpc.WithInsecure())
	if err != nil {
		logrus.WithError(err).Error("Unable to connect to runner node")
		return err
	}
	defer conn.Close()

	c := pb.NewRunnerProtocolClient(conn)

	protocolClient, err := c.Engage(context.Background())
	if err != nil {
		logrus.WithError(err).Error("Unable to create client to runner node")
		return err
	}
	err = protocolClient.Send(&pb.ClientMsg{Body: &pb.ClientMsg_Try{Try: &pb.TryCall{ModelsCallJson: ""}}})
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
