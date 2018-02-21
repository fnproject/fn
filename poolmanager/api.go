package poolmanager

import (
	"sync"
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/golang/protobuf/ptypes"

	"github.com/sirupsen/logrus"
)

type NodePoolManager interface {
	ScheduleUpdates(lbID string, agg CapacityAggregator, period time.Duration)
	GetLBGroup(lbgID string) ([]string, error)
}

type remoteNodePoolManager struct {
	serverAddr string
	client     remoteClient
}

func NewNodePoolManager(serverAddr string, cert string, key string, ca string) NodePoolManager {
	// TODO support reconnects
	c, e := newRemoteClient(serverAddr, cert, key, ca)
	if e != nil {
		logrus.Error("Failed to connect to the node pool manager for sending capacity update")
	}
	return &remoteNodePoolManager{serverAddr: serverAddr, client: c}
}

func (npm *remoteNodePoolManager) ScheduleUpdates(lbID string, agg CapacityAggregator, period time.Duration) {
	ticker := time.NewTicker(period)
	go func() {
		for _ = range ticker.C {
			snapshots := []*model.CapacitySnapshot{}
			agg.Iterate(func(lbgID string, e *CapacityEntry) {
				snapshot := &model.CapacitySnapshot{GroupId: &model.LBGroupId{Id: lbgID}, MemMbTotal: e.TotalMemoryMb}
				snapshots = append(snapshots, snapshot)
			})
			logrus.Debugf("Advertising new capacity snapshot %+v", snapshots)
			npm.client.AdvertiseCapacity(&model.CapacitySnapshotList{
				Snapshots: snapshots,
				LbId:      lbID,
				Ts:        ptypes.TimestampNow(),
			})
		}
	}()
}

func (npm *remoteNodePoolManager) GetLBGroup(lbgID string) ([]string, error) {
	m, err := npm.client.GetLBGroup(&model.LBGroupId{Id: lbgID})
	if err != nil {
		return nil, err
	}

	logrus.WithField("runners", m.GetRunners()).Debug("Received runner list")
	runnerList := make([]string, len(m.GetRunners()))
	for i, r := range m.GetRunners() {
		runnerList[i] = r.Address
	}
	return runnerList, nil
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
