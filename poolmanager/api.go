package poolmanager

import (
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/golang/protobuf/ptypes"

	"github.com/sirupsen/logrus"
)

const (
	updatesBufferSize = 10000
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

//CapacityAdvertiser allows capacity to be assigned/released and advertised to a node pool manager
type CapacityAdvertiser interface {
	AssignCapacity(r *CapacityRequest)
	ReleaseCapacity(r *CapacityRequest)
	Shutdown() error
}

type capacityEntry struct {
	TotalMemoryMb uint64
}

type CapacityRequest struct {
	TotalMemoryMb uint64
	LBGroupID     string
	assignment    bool
}

// true if this capacity requirement requires no resources
func (e *capacityEntry) isZero() bool {
	return e.TotalMemoryMb == 0
}

type queueingAdvertiser struct {
	updates  chan *CapacityRequest
	capacity map[string]*capacityEntry
	shutdown chan interface{}
	npm      NodePoolManager
	lbID     string
}

//NewCapacityAdvertiser return a CapacityAdvertiser
func NewCapacityAdvertiser(npm NodePoolManager, lbID string, period time.Duration) CapacityAdvertiser {
	agg := &queueingAdvertiser{
		updates:  make(chan *CapacityRequest, updatesBufferSize),
		capacity: make(map[string]*capacityEntry),
		shutdown: make(chan interface{}),
		npm:      npm,
		lbID:     lbID,
	}

	agg.scheduleUpdates(period)
	return agg
}

func (a *queueingAdvertiser) scheduleUpdates(period time.Duration) {
	ticker := time.NewTicker(period)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.sendAdvertisements()

			case update := <-a.updates:
				a.mergeUpdate(update)

			case <-a.shutdown:
				return

			}
		}
	}()
}

func (a *queueingAdvertiser) AssignCapacity(r *CapacityRequest) {
	a.udpateCapacity(r, true)
}

func (a *queueingAdvertiser) ReleaseCapacity(r *CapacityRequest) {
	a.udpateCapacity(r, false)
}

// don't leak implementation to caller
func (a *queueingAdvertiser) udpateCapacity(r *CapacityRequest, assignment bool) {
	if r.LBGroupID == "" {
		logrus.Warn("Missing LBG for capacity update!")
		return
	}

	r.assignment = assignment

	select {
	case a.updates <- r:
		// do not block
	default:
		logrus.Warn("Buffer size exceeded, dropping released capacity update before aggregation")
	}
}

func (a *queueingAdvertiser) mergeUpdate(r *CapacityRequest) {
	if e, ok := a.capacity[r.LBGroupID]; ok {
		if r.assignment {
			e.TotalMemoryMb += r.TotalMemoryMb
			logrus.WithField("lbg_id", r.LBGroupID).Debugf("Increased assigned capacity to %vMB", e.TotalMemoryMb)
		} else {
			e.TotalMemoryMb -= r.TotalMemoryMb
			logrus.WithField("lbg_id", r.LBGroupID).Debugf("Released assigned capacity to %vMB", e.TotalMemoryMb)
		}

	} else {
		if r.assignment {
			a.capacity[r.LBGroupID] = &capacityEntry{TotalMemoryMb: r.TotalMemoryMb}
			logrus.WithField("lbg_id", r.LBGroupID).Debugf("Assigned new capacity of %vMB", r.TotalMemoryMb)
		} else {
			logrus.WithField("lbg_id", r.LBGroupID).Warn("Attempted to release unknown assigned capacity!")
		}
	}
}

func (a *queueingAdvertiser) sendAdvertisements() {
	snapshots := []*model.CapacitySnapshot{}
	for lbgID, e := range a.capacity {
		snapshots = append(snapshots, &model.CapacitySnapshot{
			GroupId:    &model.LBGroupId{Id: lbgID},
			MemMbTotal: e.TotalMemoryMb,
		})

		// clean entries with zero capacity requirements
		// after including them in advertisement
		if e.isZero() {
			logrus.WithField("lbg_id", lbgID).Debug("Purging nil capacity requirement")
			delete(a.capacity, lbgID)
		}
	}
	// don't block consuming updates while calling out to NPM
	go func() {
		a.npm.AdvertiseCapacity(&model.CapacitySnapshotList{
			Snapshots: snapshots,
			LbId:      a.lbID,
			Ts:        ptypes.TimestampNow(),
		})
	}()
}

func (a *queueingAdvertiser) Shutdown() error {
	logrus.Info("Shutting down capacity advertiser")
	close(a.shutdown)
	return nil
}
