package poolmanager

import (
	"testing"
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
)

const (
	updatePeriod = 10 * time.Millisecond
)

type mockRemoteNodePoolManager struct {
	emptyCapacityAdv bool //this is just for testing
	capacity         map[string]uint64
}

func (n *mockRemoteNodePoolManager) AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error {

	for _, v := range snapshots.Snapshots {
		n.capacity[v.GetGroupId().GetId()] = v.GetMemMbTotal()
	}
	if len(snapshots.Snapshots) == 0 {
		//No new capacity requested we mark that we have seen it
		n.emptyCapacityAdv = true
	}

	return nil
}

func (n *mockRemoteNodePoolManager) GetRunners(lbgID string) ([]string, error) {
	return []string{}, nil
}

func (n *mockRemoteNodePoolManager) Shutdown() error { return nil }

func checkCapacity(t *testing.T, lbGroupID string, p time.Duration, npm NodePoolManager, expected uint64) {
	// give time to spawn the scheduleUpdates
	time.Sleep(2 * p)
	n, ok := npm.(*mockRemoteNodePoolManager)
	if !ok {
		t.Error("Unable to cast to the mockRemoteNodePoolManager")
	}
	actual, ok := n.capacity[lbGroupID]
	if !ok {
		t.Errorf("Unable to find capacity for lbGroupID %s", lbGroupID)
	}
	if expected != actual {
		t.Errorf("Wrong capacity reported, expected: %d got: %d", expected, actual)
	}
}

func newQueueingAdvertiser() (CapacityAdvertiser, NodePoolManager) {
	npm := &mockRemoteNodePoolManager{capacity: make(map[string]uint64)}
	adv := NewCapacityAdvertiser(npm, "lb-test", updatePeriod)
	return adv, npm
}

func TestQueueingAdvAddRelease(t *testing.T) {
	lbGroupID := "lbg-test1"
	qAdv, npm := newQueueingAdvertiser()
	defer qAdv.Shutdown()
	expected := uint64(128)
	e := &CapacityRequest{TotalMemoryMb: expected, LBGroupID: lbGroupID}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID, updatePeriod, npm, expected)

	// New Assignment
	qAdv.AssignCapacity(e)
	expected = expected + e.TotalMemoryMb
	checkCapacity(t, lbGroupID, updatePeriod, npm, expected)

	// Release capacity
	qAdv.ReleaseCapacity(e)
	expected = expected - e.TotalMemoryMb
	checkCapacity(t, lbGroupID, updatePeriod, npm, expected)
}

func TestQueueingAdvAddReleaseMultiLBGroup(t *testing.T) {
	lbGroupID1 := "lbg-test1"
	lbGroupID2 := "lbg-test2"
	qAdv, npm := newQueueingAdvertiser()
	defer qAdv.Shutdown()

	expected1 := uint64(128)
	e := &CapacityRequest{TotalMemoryMb: expected1, LBGroupID: lbGroupID1}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID1, updatePeriod, npm, expected1)

	expected2 := uint64(256)
	e = &CapacityRequest{TotalMemoryMb: expected2, LBGroupID: lbGroupID2}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID2, updatePeriod, npm, expected2)
}

func TestQueueingAdvPurgeCapacity(t *testing.T) {
	lbGroupID := "lbg-test1"
	qAdv, npm := newQueueingAdvertiser()
	defer qAdv.Shutdown()

	expected := uint64(128)
	e := &CapacityRequest{TotalMemoryMb: expected, LBGroupID: lbGroupID}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID, updatePeriod, npm, expected)

	// Release capacity
	expected = uint64(0)
	qAdv.ReleaseCapacity(e)
	// we want to assert that we have received an Adv with 0 capacity
	checkCapacity(t, lbGroupID, updatePeriod, npm, 0)
	// we expect to have advertised an empty CapacitySnapshotList
	time.Sleep(2 * updatePeriod)
	n, ok := npm.(*mockRemoteNodePoolManager)
	if !ok {
		t.Error("Unable to cast to the mockRemoteNodePoolManager")
	}
	if !n.emptyCapacityAdv {
		t.Error("Expected to have seen an empty CapacitySnapshot advertised")
	}
}
