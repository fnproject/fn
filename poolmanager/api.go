package poolmanager

import (
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/golang/protobuf/ptypes"

	"github.com/sirupsen/logrus"
)

const (
	UpdatesBufferSize = 10000
)

type NodePoolManager interface {
	AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error
	GetRunners(lbgID string) ([]string, error)
	Shutdown() error
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
	return &remoteNodePoolManager{
		serverAddr: serverAddr,
		client:     c,
	}
}

func (npm *remoteNodePoolManager) Shutdown() error {
	logrus.Info("Shutting down node pool manager client")
	return npm.client.Shutdown()
}

func (npm *remoteNodePoolManager) GetRunners(lbGroupID string) ([]string, error) {
	m, err := npm.client.GetLBGroup(&model.LBGroupId{Id: lbGroupID})
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

func (npm *remoteNodePoolManager) AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error {
	logrus.Debugf("Advertising new capacity snapshot %+v", snapshots)
	return npm.client.AdvertiseCapacity(snapshots)
}

//CapacityAggregator exposes the method to manage capacity calculation
type CapacityAggregator interface {
	ScheduleUpdates(lbID string, npm NodePoolManager, period time.Duration)
	AssignCapacity(entry *CapacityEntry)
	ReleaseCapacity(entry *CapacityEntry)
	Iterate(func(string, *CapacityEntry))
	Shutdown() error
}

type CapacityEntry struct {
	TotalMemoryMb uint64
	LBGroupID     string
	assignment    bool
}

type inMemoryAggregator struct {
	updates  chan *CapacityEntry
	capacity map[string]*CapacityEntry
	shutdown chan interface{}
}

//NewCapacityAggregator return a CapacityAggregator
func NewCapacityAggregator() CapacityAggregator {
	return &inMemoryAggregator{
		updates:  make(chan *CapacityEntry, UpdatesBufferSize),
		capacity: make(map[string]*CapacityEntry),
		shutdown: make(chan interface{}),
	}
}

func (agg *inMemoryAggregator) ScheduleUpdates(lbID string, npm NodePoolManager, period time.Duration) {
	ticker := time.NewTicker(period)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				snapshots := []*model.CapacitySnapshot{}
				agg.Iterate(func(lbgID string, e *CapacityEntry) {
					snapshot := &model.CapacitySnapshot{GroupId: &model.LBGroupId{Id: lbgID}, MemMbTotal: e.TotalMemoryMb}
					snapshots = append(snapshots, snapshot)
				})

				npm.AdvertiseCapacity(&model.CapacitySnapshotList{
					Snapshots: snapshots,
					LbId:      lbID,
					Ts:        ptypes.TimestampNow(),
				})

			case update := <-agg.updates:
				agg.mergeUpdate(update)

			case <-agg.shutdown:
				return

			}
		}
	}()
}

func (a *inMemoryAggregator) AssignCapacity(entry *CapacityEntry) {
	a.udpateCapacity(entry, true)
}

func (a *inMemoryAggregator) ReleaseCapacity(entry *CapacityEntry) {
	a.udpateCapacity(entry, false)
}

// don't leak implementation to caller
func (a *inMemoryAggregator) udpateCapacity(entry *CapacityEntry, assignment bool) {
	if entry.LBGroupID == "" {
		logrus.Warn("Missing LBG for capacity update!")
		return
	}

	entry.assignment = assignment

	select {
	case a.updates <- entry:
		// do not block
	default:
		logrus.Warn("Buffer size exceeded, dropping released capacity update before aggregation")
	}
}

func (a *inMemoryAggregator) mergeUpdate(entry *CapacityEntry) {
	if v, ok := a.capacity[entry.LBGroupID]; ok {
		if entry.assignment {
			v.TotalMemoryMb += entry.TotalMemoryMb
			logrus.WithField("lbg_id", entry.LBGroupID).Debugf("Increased assigned capacity to %vMB", v.TotalMemoryMb)
		} else {
			v.TotalMemoryMb -= entry.TotalMemoryMb
			logrus.WithField("lbg_id", entry.LBGroupID).Debugf("Released assigned capacity to %vMB", v.TotalMemoryMb)
		}

	} else {
		if entry.assignment {
			a.capacity[entry.LBGroupID] = &CapacityEntry{TotalMemoryMb: entry.TotalMemoryMb}
			logrus.WithField("lbg_id", entry.LBGroupID).Debugf("Assigned new capacity of %vMB", entry.TotalMemoryMb)
		} else {
			logrus.WithField("lbg_id", entry.LBGroupID).Warn("Attempted to release unknown assigned capacity!")
		}
	}
}

func (a *inMemoryAggregator) Iterate(fn func(string, *CapacityEntry)) {
	for k, v := range a.capacity {
		fn(k, v)
	}
}

func (a *inMemoryAggregator) Shutdown() error {
	logrus.Info("Shutting down capacity aggregator")
	close(a.shutdown)
	return nil
}
