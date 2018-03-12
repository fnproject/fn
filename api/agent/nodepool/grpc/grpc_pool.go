package grpc

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/fnproject/fn/api/agent"
	pb "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/grpcutil"
	"github.com/fnproject/fn/poolmanager"
	"github.com/sirupsen/logrus"
)

const (
	// CapacityUpdatePeriod defines how often the capacity updates are sent
	CapacityUpdatePeriod = 1 * time.Second
)

type gRPCNodePool struct {
	npm        poolmanager.NodePoolManager
	advertiser poolmanager.CapacityAdvertiser
	mx         sync.RWMutex
	lbg        map[string]*lbg // {lbgid -> *lbg}
	generator  secureRunnerFactory
	//TODO find a better place for this
	pki *pkiData

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
	runners map[string]agent.Runner
	r_list  atomic.Value // We attempt to maintain the same order of runners as advertised by the NPM.
	// This is to preserve as reasonable behaviour as possible for the CH algorithm
	generator secureRunnerFactory
}

type nullRunner struct{}

func (n *nullRunner) TryExec(ctx context.Context, call agent.Call) (bool, error) {
	return false, nil
}

func (n *nullRunner) Close() {}

func (n *nullRunner) Address() string {
	return ""
}

var nullRunnerSingleton = new(nullRunner)

type gRPCRunner struct {
	// Need a WaitGroup of TryExec in flight
	wg      sync.WaitGroup
	address string
	conn    *grpc.ClientConn
	client  pb.RunnerProtocolClient
}

// allow factory to be overridden in tests
type secureRunnerFactory func(addr string, cert string, key string, ca string) (agent.Runner, error)

func secureGRPCRunnerFactory(addr string, cert string, key string, ca string) (agent.Runner, error) {
	p := &pkiData{
		cert: cert,
		key:  key,
		ca:   ca,
	}
	conn, client, err := runnerConnection(addr, p)
	if err != nil {
		return nil, err
	}

	return &gRPCRunner{
		address: addr,
		conn:    conn,
		client:  client,
	}, nil
}

func DefaultgRPCNodePool(npmAddress string, cert string, key string, ca string) agent.NodePool {
	npm := poolmanager.NewNodePoolManager(npmAddress, cert, key, ca)
	// TODO do we need to persistent this ID in order to survive restart?
	lbID := id.New().String()
	advertiser := poolmanager.NewCapacityAdvertiser(npm, lbID, CapacityUpdatePeriod)
	return newgRPCNodePool(cert, key, ca, npm, advertiser, secureGRPCRunnerFactory)
}

func newgRPCNodePool(cert string, key string, ca string, npm poolmanager.NodePoolManager, advertiser poolmanager.CapacityAdvertiser, rf secureRunnerFactory) agent.NodePool {

	logrus.Info("Starting dynamic runner pool")
	p := &pkiData{
		ca:   ca,
		cert: cert,
		key:  key,
	}

	np := &gRPCNodePool{
		npm:        npm,
		advertiser: advertiser,
		lbg:        make(map[string]*lbg),
		generator:  rf,
		shutdown:   make(chan struct{}),
		pki:        p,
	}
	go np.maintenance()
	return np
}

func (np *gRPCNodePool) Runners(lbgID string) []agent.Runner {
	np.mx.RLock()
	lbg, ok := np.lbg[lbgID]
	np.mx.RUnlock()

	if !ok {
		np.mx.Lock()
		lbg, ok = np.lbg[lbgID]
		if !ok {
			lbg = newLBG(lbgID, np.generator)
			np.lbg[lbgID] = lbg
		}
		np.mx.Unlock()
	}

	return lbg.runnerList()
}

func (np *gRPCNodePool) Shutdown() {
	np.advertiser.Shutdown()
	np.npm.Shutdown()
}

func (np *gRPCNodePool) AssignCapacity(r *poolmanager.CapacityRequest) {
	np.advertiser.AssignCapacity(r)

}
func (np *gRPCNodePool) ReleaseCapacity(r *poolmanager.CapacityRequest) {
	np.advertiser.ReleaseCapacity(r)
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

func newLBG(lbgID string, generator secureRunnerFactory) *lbg {
	lbg := &lbg{
		id:        lbgID,
		runners:   make(map[string]agent.Runner),
		r_list:    atomic.Value{},
		generator: generator,
	}
	lbg.r_list.Store([]agent.Runner{})
	return lbg
}

func (np *gRPCNodePool) reloadLBGmembership() {
	np.mx.RLock()
	lbgroups := np.lbg
	np.mx.RUnlock()
	for lbgID, lbg := range lbgroups {
		lbg.reloadMembers(lbgID, np.npm, np.pki)
	}
}

func (lbg *lbg) runnerList() []agent.Runner {
	orig_runners := lbg.r_list.Load().([]agent.Runner)
	// XXX: Return a copy. If we required this to be immutably read by the caller, we could return the structure directly
	runners := make([]agent.Runner, len(orig_runners))
	copy(runners, orig_runners)
	return runners
}

func (lbg *lbg) reloadMembers(lbgID string, npm poolmanager.NodePoolManager, p *pkiData) {

	runners, err := npm.GetRunners(lbgID)
	if err != nil {
		logrus.Debug("Failed to get the list of runners from node pool manager")
	}
	lbg.mx.Lock()
	defer lbg.mx.Unlock()
	r_list := make([]agent.Runner, len(runners))
	seen := map[string]bool{} // If we've seen a particular runner or not
	var errGenerator error
	for i, addr := range runners {
		r, ok := lbg.runners[addr]
		if !ok {
			logrus.WithField("runner_addr", addr).Debug("New Runner to be added")
			r, errGenerator = lbg.generator(addr, p.cert, p.key, p.ca)
			if errGenerator != nil {
				logrus.WithField("runner_addr", addr).Debug("Creation of the new runner failed")
			} else {
				lbg.runners[addr] = r
			}
		}
		if errGenerator == nil {
			r_list[i] = r // Maintain the delivered order
		} else {
			// some algorithms (like consistent hash) work better if the i'th element
			// of r_list points to the same node on all LBs, so insert a placeholder
			// if we can't create the runner for some reason"
			r_list[i] = nullRunnerSingleton
		}

		seen[addr] = true
	}
	lbg.r_list.Store(r_list)

	// Remove any runners that we have not encountered
	for addr, r := range lbg.runners {
		if _, ok := seen[addr]; !ok {
			logrus.WithField("runner_addr", addr).Debug("Removing drained runner")
			delete(lbg.runners, addr)
			r.Close()
		}
	}
}

func (r *gRPCRunner) Close() {
	go func() {
		r.wg.Wait()
		r.conn.Close()
	}()
}

func runnerConnection(address string, pki *pkiData) (*grpc.ClientConn, pb.RunnerProtocolClient, error) {
	ctx := context.Background()

	var creds credentials.TransportCredentials
	if pki != nil {
		var err error
		creds, err = grpcutil.CreateCredentials(pki.cert, pki.key, pki.ca)
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

func (r *gRPCRunner) TryExec(ctx context.Context, call agent.Call) (bool, error) {
	logrus.WithField("runner_addr", r.address).Debug("Attempting to place call")
	r.wg.Add(1)
	defer r.wg.Done()

	// Get app and route information
	// Construct model.Call with CONFIG in it already
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

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, call agent.Call, done chan error) {
	w, err := agent.ResponseWriter(&call)

	if err != nil {
		logrus.WithError(err).Error("Unable to get response writer from call")
		done <- err
		return
	}

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
