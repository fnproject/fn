package agent

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"

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

// NodePool is the interface to interact with Node pool manager
type NodePool interface {
	Runners(lbgID string) []Runner
	AssignCapacity(capacity *poolmanager.CapacityEntry)
	ReleaseCapacity(capacity *poolmanager.CapacityEntry)
	Shutdown()
}

// Runner is the interface to invoke the execution of a function call on a specific runner
type Runner interface {
	TryExec(ctx context.Context, call Call) (bool, error)
	Close()
}

// RunnerFactory is a factory func that creates a Runner usable by the pool.
type RunnerFactory func(addr string, lbgId string, pki pkiData) (Runner, error)

type gRPCNodePool struct {
	npm        poolmanager.NodePoolManager
	aggregator poolmanager.CapacityAggregator
	mx         sync.RWMutex
	lbg        map[string]*lbg // {lbgid -> *lbg}
	generator  RunnerFactory
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
	runners map[string]Runner
	r_list  atomic.Value // We attempt to maintain the same order of runners as advertised by the NPM.
	// This is to preserve as reasonable behaviour as possible for the CH algorithm
	generator RunnerFactory
}

type gRPCRunner struct {
	// Need a WaitGroup of TryExec in flight
	wg      sync.WaitGroup
	address string
	conn    *grpc.ClientConn
	client  pb.RunnerProtocolClient
}

func GRPCRunnerFactory(addr string, lbgID string, p pkiData) (Runner, error) {
	conn, client := runnerConnection(addr, lbgID, p)
	return &gRPCRunner{
		address: addr,
		conn:    conn,
		client:  client,
	}, nil
}

func DefaultgRPCNodePool(npmAddress string, cert string, key string, ca string) NodePool {
	npm := poolmanager.NewNodePoolManager(npmAddress, cert, key, ca)
	aggregator := poolmanager.NewCapacityAggregator()
	rf := GRPCRunnerFactory

	return NewgRPCNodePool(cert, key, ca, npm, aggregator, rf)
}

func NewgRPCNodePool(cert string, key string, ca string,
	npm poolmanager.NodePoolManager,
	aggregator poolmanager.CapacityAggregator,
	rf RunnerFactory) NodePool {
	p := pkiData{
		ca:   ca,
		cert: cert,
		key:  key,
	}

	np := &gRPCNodePool{
		npm:        npm,
		aggregator: aggregator,
		lbg:        make(map[string]*lbg),
		generator:  rf,
		shutdown:   make(chan struct{}),
		pki:        p,
	}
	go np.maintenance()
	// TODO do we need to persistent this ID in order to survive restart?
	lbID := id.New().String()

	np.aggregator.ScheduleUpdates(lbID, np.npm, CapacityUpdatePeriod)
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
			lbg = newLBG(lbgID, np.generator)
			np.lbg[lbgID] = lbg
		}
		np.mx.Unlock()
	}

	return lbg.runnerList()
}

func (np *gRPCNodePool) Shutdown() {
	np.aggregator.Shutdown()
	np.npm.Shutdown()
}

func (np *gRPCNodePool) AssignCapacity(capacity *poolmanager.CapacityEntry) {
	np.aggregator.AssignCapacity(capacity)

}
func (np *gRPCNodePool) ReleaseCapacity(capacity *poolmanager.CapacityEntry) {
	np.aggregator.ReleaseCapacity(capacity)
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

func newLBG(lbgID string, generator RunnerFactory) *lbg {
	lbg := &lbg{
		id:        lbgID,
		runners:   make(map[string]Runner),
		r_list:    atomic.Value{},
		generator: generator,
	}
	lbg.r_list.Store([]Runner{})
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

func (lbg *lbg) runnerList() []Runner {
	orig_runners := lbg.r_list.Load().([]Runner)
	// XXX: Return a copy. If we required this to be immutably read by the caller, we could return the structure directly
	runners := make([]Runner, len(orig_runners))
	for i, r := range orig_runners {
		runners[i] = r
	}
	return runners
}

func (lbg *lbg) reloadMembers(lbgID string, npm poolmanager.NodePoolManager, p pkiData) {

	runners, err := npm.GetRunners(lbgID)
	if err != nil {
		logrus.Debug("Failed to get the list of runners from node pool manager")
	}
	lbg.mx.Lock()
	defer lbg.mx.Unlock()
	r_list := make([]Runner, len(runners))
	seen := map[string]bool{} // If we've seen a particular runner or not
	for i, addr := range runners {
		r, ok := lbg.runners[addr]
		if !ok {
			logrus.WithField("runner_addr", addr).Debug("New Runner to be added")
			var err error
			r, err = lbg.generator(addr, lbgID, p)
			if err != nil {
				logrus.WithField("runner_addr", addr).Debug("Creation of the new runner failed")
			} else {
				lbg.runners[addr] = r
			}
		}
		if r != nil {
			r_list[i] = r // Maintain the delivered order
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

func runnerConnection(address, lbGroupID string, pki pkiData) (*grpc.ClientConn, pb.RunnerProtocolClient) {
	ctx := context.Background()

	creds, err := grpcutil.CreateCredentials(pki.cert, pki.key, pki.ca)
	if err != nil {
		logrus.WithError(err).Error("Unable to create credentials to connect to runner node")

	}
	conn, err := grpcutil.DialWithBackoff(ctx, address, creds, grpc.DefaultBackoffConfig)

	if err != nil {
		logrus.WithError(err).Error("Unable to connect to runner node")
	}

	protocolClient := pb.NewRunnerProtocolClient(conn)
	logrus.WithField("lbg_id", lbGroupID).WithField("runner_addr", address).Info("Connected to runner")

	return conn, protocolClient
}

func (r *gRPCRunner) TryExec(ctx context.Context, call Call) (bool, error) {
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
		}
		logrus.Info("Runner committed invocation request, sending data frames")
		done := make(chan error)
		go receiveFromRunner(runnerConnection, call, done)
		sendToRunner(call, runnerConnection)
		return true, <-done

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

func receiveFromRunner(protocolClient pb.RunnerProtocol_EngageClient, call Call, done chan error) {
	w, err := ResponseWriter(&call)

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
