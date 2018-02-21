package poolmanager

import (
	"context"
	"sync"
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
	ptypes "github.com/golang/protobuf/ptypes"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Client interface {
	AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error
	GetLBGroupMembership(id *model.LBGroupId) (*model.LBGroupMembership, error)
	Shutdown() error
}

//CapacityAggregator exposes the method to manage capacity calculation
type CapacityAggregator interface {
	AssignCapacity(entry *CapacityEntry, lbgID string)
	ReleaseCapacity(entry *CapacityEntry, lbgID string)
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

func CapacityUpdatesSchedule(serverAddr, lbID string, agg CapacityAggregator, period time.Duration) {
	// TODO support reconnects
	c, e := NewClient(serverAddr)
	if e != nil {
		logrus.Error("Failed to connect to the node pool manager for sending capacity update")
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
			logrus.Debugf("Advertising new capacity snapshot %+v", snapshots)
			c.AdvertiseCapacity(&model.CapacitySnapshotList{
				Snapshots: snapshots,
				LbId:      lbID,
				Ts:        ptypes.TimestampNow(),
			})
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

func (a *inMemoryAggregator) AssignCapacity(entry *CapacityEntry, lbgID string) {
	//TODO is it possible to use prometheus gauge? definitely to Inc and Dec but how to read
	a.capMtx.Lock()
	defer a.capMtx.Unlock()

	if v, ok := a.capacity[lbgID]; ok {
		v.TotalMemoryMb += entry.TotalMemoryMb
	} else {
		a.capacity[lbgID] = &CapacityEntry{TotalMemoryMb: entry.TotalMemoryMb}
	}
}

func (a *inMemoryAggregator) ReleaseCapacity(entry *CapacityEntry, lbgID string) {
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
		logrus.Fatalf("Failed to push snapshots %v", err)
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
