package poolmanager

import (
	"context"
	"log"
	"sync"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"google.golang.org/grpc"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/hybrid"
)

type Client interface {
	AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error
	GetLBGroupMembership(id *model.LBGroupId) (*model.LBGroupMembership, error)
	Shutdown() error
}

//CapacityAggregator exposes the method to manage capacity calculation
type CapacityAggregator interface {
	AddCapacity(entry *CapacityEntry, id *model.LBGroupId)
	RemoveCapacity(entry *CapacityEntry, id *model.LBGroupId)
}

type CapacityEntry struct {
	TotalMemoryMb uint64
}

type inMemoryAggregator struct {
	capacity map[*model.LBGroupId]*CapacityEntry
	capMtx   *sync.RWMutex
}
type grpcPoolManagerClient struct {
	scaler  model.NodePoolScalerClient
	manager model.RunnerManagerClient
	agent   agent.DataAccess
	conn    *grpc.ClientConn
}

func NewClient(serverAddr string) (Client, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure()}
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	agent, err := hybrid.NewClient("localhost:8888")
	if err != nil {
		return nil, err
	}

	return &grpcPoolManagerClient{
		agent:   agent,
		scaler:  model.NewNodePoolScalerClient(conn),
		manager: model.NewRunnerManagerClient(conn),
		conn:    conn,
	}, nil
}

//NewCapacityAggregator return a CapacityAggregator
func NewCapacityAggregator() CapacityAggregator {
	return &inMemoryAggregator{
		capacity: make(map[*model.LBGroupId]*CapacityEntry),
		capMtx:   &sync.RWMutex{},
	}
}

func (a *inMemoryAggregator) AddCapacity(entry *CapacityEntry, id *model.LBGroupId) {
	//TODO is it possible to use prometheus gauge? definitely to Inc and Dec but how to read
	a.capMtx.Lock()
	defer a.capMtx.Unlock()

	if v, ok := a.capacity[id]; ok {
		v.TotalMemoryMb += entry.TotalMemoryMb
	} else {
		a.capacity[id] = &CapacityEntry{TotalMemoryMb: entry.TotalMemoryMb}
	}
}

func (a *inMemoryAggregator) RemoveCapacity(entry *CapacityEntry, id *model.LBGroupId) {
	a.capMtx.Lock()
	defer a.capMtx.Unlock()

	if v, ok := a.capacity[id]; ok {
		v.TotalMemoryMb -= entry.TotalMemoryMb
	}
}

func (c *grpcPoolManagerClient) AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error {
	_, err := c.scaler.AdvertiseCapacity(context.Background(), snapshots)
	if err != nil {
		log.Fatalf("Failed to push snapshots %v", err)
		return err
	}
	return nil
}

func (c *grpcPoolManagerClient) GetLBGroupMembership(id *model.LBGroupId) (*model.LBGroupMembership, error) {
	return c.manager.GetLBGroup(context.Background(), id)
}

func (c *grpcPoolManagerClient) Shutdown() error {
	return c.conn.Close()
}
