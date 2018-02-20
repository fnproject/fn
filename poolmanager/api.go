package poolmanager

import (
	"context"
	"log"
	"sync"
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"google.golang.org/grpc"
)

type Client interface {
	AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error
	GetLBGroupMembership(id *model.LBGroupId) (*model.LBGroupMembership, error)
	Shutdown() error
}

//CapacityAggregator exposes the method to manage capacity calculation
type CapacityAggregator interface {
	AddCapacity(entry *CapacityEntry, lbgID string)
	RemoveCapacity(entry *CapacityEntry, lbgID string)
	Iterate(func(string, *CapacityEntry))
}

type CapacityEntry struct {
	TotalMemoryMb uint64
}

type inMemoryAggregator struct {
	capacity map[string]*CapacityEntry
	capMtx   *sync.RWMutex
}
type grpcPoolManagerClient struct {
	scaler  model.NodePoolScalerClient
	manager model.RunnerManagerClient
	conn    *grpc.ClientConn
}

func CapacityUpdatesSchedule(serverAddr string, agg CapacityAggregator, period time.Duration) {
	// TODO support reconnects
	c, e := NewClient(serverAddr)
	if e != nil {
		return
	}

	ticker := time.NewTicker(period)
	go func() {
		for _ = range ticker.C {
			var snapshots []*model.CapacitySnapshot
			agg.Iterate(func(lbgID string, e *CapacityEntry) {
				snapshot := &model.CapacitySnapshot{GroupId: &model.LBGroupId{Id: lbgID}, MemMbTotal: e.TotalMemoryMb}
				snapshots = append(snapshots, snapshot)
			})
			// TODO missing ts, etc
			c.AdvertiseCapacity(&model.CapacitySnapshotList{Snapshots: snapshots})
		}
	}()
}

func NewClient(serverAddr string) (Client, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure()}
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return &grpcPoolManagerClient{
		scaler:  model.NewNodePoolScalerClient(conn),
		manager: model.NewRunnerManagerClient(conn),
		conn:    conn,
	}, nil
}

//NewCapacityAggregator return a CapacityAggregator
func NewCapacityAggregator() CapacityAggregator {
	return &inMemoryAggregator{
		capacity: make(map[string]*CapacityEntry),
		capMtx:   &sync.RWMutex{},
	}
}

func (a *inMemoryAggregator) AddCapacity(entry *CapacityEntry, lbgID string) {
	//TODO is it possible to use prometheus gauge? definitely to Inc and Dec but how to read
	a.capMtx.Lock()
	defer a.capMtx.Unlock()

	if v, ok := a.capacity[lbgID]; ok {
		v.TotalMemoryMb += entry.TotalMemoryMb
	} else {
		a.capacity[lbgID] = &CapacityEntry{TotalMemoryMb: entry.TotalMemoryMb}
	}
}

func (a *inMemoryAggregator) RemoveCapacity(entry *CapacityEntry, lbgID string) {
	a.capMtx.Lock()
	defer a.capMtx.Unlock()

	if v, ok := a.capacity[lbgID]; ok {
		v.TotalMemoryMb -= entry.TotalMemoryMb
	}
}

func (a *inMemoryAggregator) Iterate(fn func(string, *CapacityEntry)) {
	a.capMtx.Lock()
	defer a.capMtx.Unlock()

	for k, v := range a.capacity {
		fn(k, v)
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
