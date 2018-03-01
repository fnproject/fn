package poolmanager

import (
	"testing"
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
)

const (
	updatePeriod = 10 * time.Millisecond
)

type mockRemoteNodePoolManager struct{}

func (n *mockRemoteNodePoolManager) AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error {
	return nil
}

func (n *mockRemoteNodePoolManager) GetRunners(lbgID string) ([]string, error) {
	return []string{}, nil
}

func (n *mockRemoteNodePoolManager) Shutdown() error { return nil }

func checkCapacity(t *testing.T, lbGroupID string, p time.Duration, qAdv *queueingAdvertiser, expected uint64) {
	// give time to spawn the scheduleUpdates
	time.Sleep(2 * p)
	actual, ok := qAdv.capacity[lbGroupID]
	if !ok {
		t.Errorf("Unable to find capacity for lbGroupID %s", lbGroupID)
	}
	if expected != actual.TotalMemoryMb {
		t.Errorf("Wrong capacity reported, expected: %d got: %d", expected, actual)
	}
}

func newQueueingAdvertiser(t *testing.T) *queueingAdvertiser {
	npm := &mockRemoteNodePoolManager{}
	adv := NewCapacityAdvertiser(npm, "lb-test", updatePeriod)
	qAdv, ok := adv.(*queueingAdvertiser)
	if !ok {
		t.Error("Unable to cast to the queuingAdvertiser")
	}
	return qAdv
}

func TestQueueingAdvAddRelease(t *testing.T) {
	lbGroupID := "lbg-test1"
	qAdv := newQueueingAdvertiser(t)
	defer qAdv.Shutdown()
	expected := uint64(128)
	e := &CapacityRequest{TotalMemoryMb: expected, LBGroupID: lbGroupID}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID, updatePeriod, qAdv, expected)

	// New Assignment
	qAdv.AssignCapacity(e)
	expected = expected + e.TotalMemoryMb
	checkCapacity(t, lbGroupID, updatePeriod, qAdv, expected)

	// Release capacity
	qAdv.ReleaseCapacity(e)
	expected = expected - e.TotalMemoryMb
	checkCapacity(t, lbGroupID, updatePeriod, qAdv, expected)
}

func TestQueueingAdvAddReleaseMultiLBGroup(t *testing.T) {
	lbGroupID1 := "lbg-test1"
	lbGroupID2 := "lbg-test2"
	qAdv := newQueueingAdvertiser(t)
	defer qAdv.Shutdown()

	expected1 := uint64(128)
	e := &CapacityRequest{TotalMemoryMb: expected1, LBGroupID: lbGroupID1}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID1, updatePeriod, qAdv, expected1)

	expected2 := uint64(256)
	e = &CapacityRequest{TotalMemoryMb: expected2, LBGroupID: lbGroupID2}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID2, updatePeriod, qAdv, expected2)
}

func TestQueueingAdvPurgeCapacity(t *testing.T) {
	lbGroupID := "lbg-test1"
	qAdv := newQueueingAdvertiser(t)
	defer qAdv.Shutdown()

	expected := uint64(128)
	e := &CapacityRequest{TotalMemoryMb: expected, LBGroupID: lbGroupID}
	qAdv.AssignCapacity(e)
	checkCapacity(t, lbGroupID, updatePeriod, qAdv, expected)

	// Release capacity
	expected = uint64(0)
	qAdv.ReleaseCapacity(e)
	time.Sleep(2 * updatePeriod)
	_, ok := qAdv.capacity[lbGroupID]
	if ok {
		t.Error("nil capacity requirement should be purged")
	}
}
