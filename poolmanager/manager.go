package poolmanager

import (
	"time"

	"context"
	"math"
	"sync"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/fnproject/fn/poolmanager/server/cp"
	"github.com/sirupsen/logrus"
)

type CapacityManager interface {
	LBGroup(lbgid string) LBGroup
	Merge(*model.CapacitySnapshotList)
}

type LBGroup interface {
	Id() string
	UpdateRequirements(lb string, total int64)
	Purge(time.Time, func(LBGroup, string)) int64 // Remove outdated requirements, return updated value
	GetMembers() []string                         // Return *ACTIVE* members
}

type capacityManager struct {
	ctx context.Context
	mx  sync.RWMutex
	cp  cp.ControlPlane
	lbg map[string]LBGroup
}

func NewCapacityManager(ctx context.Context, cp cp.ControlPlane) CapacityManager {
	return &capacityManager{
		ctx: ctx,
		cp:  cp,
		lbg: make(map[string]LBGroup),
	}
}

func (m *capacityManager) LBGroup(lbgid string) LBGroup {
	m.mx.RLock()
	// Optimistic path
	if lbg, ok := m.lbg[lbgid]; ok {
		m.mx.RUnlock()
		return lbg
	}

	// We don't have one: upgrade the lock and allocate
	m.mx.RUnlock()
	m.mx.Lock()
	defer m.mx.Unlock()
	// Need to check again
	if lbg, ok := m.lbg[lbgid]; ok {
		return lbg
	}
	logrus.Infof("Making new LBG to handle %v", lbgid)
	lbg := newLBGroup(lbgid, m.ctx, m.cp)
	m.lbg[lbgid] = lbg
	return lbg
}

func (m *capacityManager) Merge(list *model.CapacitySnapshotList) {
	lbid := list.GetLbId()
	for _, new_req := range list.Snapshots {
		lbg := new_req.GetGroupId().GetId()

		logrus.Debugf("Merging snapshot %+v for %v from %v", new_req, lbg, lbid)
		m.LBGroup(lbg).UpdateRequirements(lbid, int64(new_req.GetMemMbTotal()))
	}
}

type lbGroup struct {
	ctx context.Context

	id string

	// Attributes for managing incoming capacity requirements
	cap_mx sync.RWMutex

	total_wanted int64
	requirements map[string]*requirement // NuLB id -> (ts, total_wanted)

	controlStream chan requirement

	// Attributes for managing runner pool membership
	run_mx sync.RWMutex
	cp     cp.ControlPlane

	current_capacity int64              // Of all active runners
	target_capacity  int64              // All active runners plus any we've already asked for
	runners          map[string]*runner // A map of everything we know about
	active_runners   []*runner          // Everything currently in use
	draining_runners []*runner          // We keep tabs on these separately
	dead_runners     []*runner          // Waiting for control plane to remove
}

type requirement struct {
	ts           time.Time // Time of last update
	total_wanted int64
}

const (
	RUNNER_ACTIVE   = iota
	RUNNER_DRAINING = iota
	RUNNER_DEAD     = iota
)

type runner struct {
	id       string // The same address may get recycled; we'll need to disambiguate somehow.
	address  string
	status   int
	capacity int64

	// XXX: If we're draining, this is handy to simulate runner readiness for shutdown
	kill_after time.Time
}

func newLBGroup(lbgid string, ctx context.Context, cp cp.ControlPlane) LBGroup {
	lbg := &lbGroup{
		ctx:          ctx,
		id:           lbgid,
		requirements: make(map[string]*requirement),
		controlStream: make(chan requirement),
		cp:           cp,
		runners:	  make(map[string]*runner),
	}
	go lbg.control()
	return lbg
}

func (lbg *lbGroup) Id() string {
	return lbg.id
}

func (lbg *lbGroup) UpdateRequirements(lb string, total int64) {
	logrus.Debugf("Updating capacity requirements for %v, lb=%v", lbg.Id(), lb)
	defer logrus.Debugf("Updated %v, lb=%v", lbg.Id(), lb)
	lbg.cap_mx.Lock()

	last, ok := lbg.requirements[lb]

	// Add in the new requirements, removing the old ones if required.
	if !ok {
		// This is a new NuLB that we're just learning about
		last = &requirement{}
		lbg.requirements[lb] = last
	}

	// Update totals: remove this LB's previous capacity assertions
	lbg.total_wanted -= last.total_wanted

	// Update totals: add this LB's new assertions and record them
	lbg.total_wanted += total

	// Keep a copy of this requirement
	now := time.Now()
	last.ts = now
	last.total_wanted = total

	// TODO: new_req also has a generation for the runner information that LB held. If that's out of date, signal that we need to readvertise

	// Send a new signal to the capacity control loop
	lbg.cap_mx.Unlock()

	logrus.Debugf("Sending new capacity requirement of %v", lbg.total_wanted)
	lbg.controlStream <- requirement{ts: now, total_wanted: lbg.total_wanted}
}

func (lbg *lbGroup) Purge(oldest time.Time, cb func(LBGroup, string)) int64 {
	lbg.cap_mx.Lock()
	defer lbg.cap_mx.Unlock()

	for lb, req := range lbg.requirements {
		if req.ts.Before(oldest) {
			// We need to nix this entry, it's utterly out-of-date
			lbg.total_wanted -= req.total_wanted
			delete(lbg.requirements, lb)

			// TODO: use a callback here to handle the deletion?
			cb(lbg, lb)
		}
	}
	return lbg.total_wanted
}

const PURGE_INTERVAL = 5 * time.Second
const VALID_REQUEST_LIFETIME = 500 * time.Millisecond
const POLL_INTERVAL = time.Second
const LARGEST_REQUEST_AT_ONCE = 20

const MAX_DRAINDOWN_LIFETIME = 50 * time.Second  // For the moment.

func (lbg *lbGroup) control() {
	// Control loop. This should receive a series of requirements.
	// Occasionally, we walk the set of LBs that have spoken to us, purging those that are out-of-date
	lastPurge := time.Now()
	nextPurge := lastPurge.Add(PURGE_INTERVAL)

	nextPoll := lastPurge.Add(POLL_INTERVAL)

	for {
		logrus.Debugf("In capacity management loop for %v", lbg.Id())
		select {
		// Manage capacity requests
		case <-time.After(nextPurge.Sub(time.Now())):
			logrus.Debugf("Purging for %v", lbg.Id())
			need := lbg.Purge(lastPurge, func(lbg LBGroup, lb string) {
				logrus.Warnf("Purging LB %v from %v - no communication received", lb, lbg.Id())
			})
			lastPurge := time.Now()
			nextPurge = lastPurge.Add(PURGE_INTERVAL)
			lbg.target(lastPurge, need)
			logrus.Debugf("Purged for %v", lbg.Id())

		case req := <-lbg.controlStream:
			logrus.Debugf("New requirement received by control loop for %v", req.total_wanted)
			lbg.target(req.ts, req.total_wanted)
			logrus.Debugf("New requirement handled", lbg.Id())

		// Poll CP for runners (this will change, it's a stub)
		case <-time.After(nextPoll.Sub(time.Now())):
			logrus.Debugf("Polling for runners for %v", lbg.Id())
			lbg.pollForRunners()
			nextPoll = time.Now().Add(POLL_INTERVAL)
			logrus.Debugf("Polled for %v", lbg.Id())
		}
	}
}

func (lbg *lbGroup) target(ts time.Time, target int64) {
	if time.Now().Sub(ts) > VALID_REQUEST_LIFETIME {
		// We have a request that's too old; drop it.
		logrus.Warnf("Request for capacity is too old: %v", ts)
		return
	}

	lbg.run_mx.Lock()
	defer lbg.run_mx.Unlock()

	// We have:
	// - total capacity in active runners
	// - required total capacity
	// - capacity per runner
	// - any additional capacity we've already asked for

	// We scale appropriately.
	if target > lbg.target_capacity {
		// Scale up.
		// Even including capacity we are expecting to come down the pipe, we don't have enough stuff.

		// Begin by reactivating any runners we're currently draining down.
		for target > lbg.target_capacity && len(lbg.draining_runners) > 0 {
			// Begin with the one we started draining last.
			runner := lbg.draining_runners[len(lbg.draining_runners)-1]
			logrus.Infof("Recovering runner %v at %v from draindown", runner.id, runner.address)

			lbg.draining_runners = lbg.draining_runners[:len(lbg.draining_runners)-1]
			runner.status = RUNNER_ACTIVE
			lbg.active_runners = append(lbg.active_runners, runner)
			lbg.current_capacity += runner.capacity
			lbg.target_capacity += runner.capacity
		}

		if target > lbg.target_capacity {
			// We still need additional capacity
			wanted := math.Min(math.Ceil(float64(target-lbg.target_capacity)/cp.CAPACITY_PER_RUNNER), LARGEST_REQUEST_AT_ONCE)
			asked_for, err := lbg.cp.ProvisionRunners(lbg.Id(), int(wanted)) // Send the request; they'll show up later
			if err != nil {
				// Some kind of error during attempt to scale up
				logrus.Errorf("Error occured during attempt to scale up: %v", err)
				return
			}
			lbg.target_capacity += int64(asked_for) * cp.CAPACITY_PER_RUNNER
		}

	} else if target <= lbg.current_capacity-cp.CAPACITY_PER_RUNNER {
		// Scale down.
		// We pick a node to turn off and move it to the draining pool.
		for target <= lbg.current_capacity-cp.CAPACITY_PER_RUNNER && len(lbg.active_runners) > 0 {
			// Begin with the one we added last.
			runner := lbg.active_runners[len(lbg.active_runners)-1]
			logrus.Infof("Marking runner %v at %v for draindown", runner.id, runner.address)

			lbg.active_runners = lbg.active_runners[:len(lbg.active_runners)-1]
			runner.status = RUNNER_DRAINING
			runner.kill_after = time.Now().Add(MAX_DRAINDOWN_LIFETIME)
			lbg.draining_runners = append(lbg.draining_runners, runner)
			lbg.current_capacity -= runner.capacity
			lbg.target_capacity -= runner.capacity
		}
	}
}

// Pool membership management
func (lbg *lbGroup) GetMembers() []string {
	lbg.run_mx.RLock()
	defer lbg.run_mx.RUnlock()

	members := make([]string, len(lbg.active_runners))
	for i, runner := range lbg.active_runners {
		members[i] = runner.address
	}
	return members
}

// Three things handled here.
// First, if any drained runners are due to die, shut them off.
// Secondly, if the CP supplies any new capacity, add that the to pool as active.
// Finally, if dead runners have been shut down, remove them
func (lbg *lbGroup) pollForRunners() {
	lbg.run_mx.Lock()
	defer lbg.run_mx.Unlock()

	now := time.Now()
	// The oldest draining runner will be in the front of the pipe.
	for len(lbg.draining_runners) > 0 && now.After(lbg.draining_runners[0].kill_after) {
		// Mark this runner as to be killed
		runner := lbg.draining_runners[0]
		logrus.Infof("Drain down for runner %v at %v complete: signalling shutdown", runner.id, runner.address)
		lbg.draining_runners = lbg.draining_runners[1:]
		runner.status = RUNNER_DEAD
		lbg.dead_runners = append(lbg.dead_runners, runner)
		if err := lbg.cp.RemoveRunner(lbg.Id(), runner.id); err != nil {
			logrus.Errorf("Error attempting to close down runner %v at %v: %v", runner.id, runner.address, err)
		}
	}

	// Get CP status and process it. This might be smarter but for the moment we just loop over everything we're told.
	logrus.Debugf("Getting hosts from ControlPlane for %v", lbg.Id())
	latestHosts, err := lbg.cp.GetLBGRunners(lbg.Id())
	if err != nil {
		logrus.Errorf("Problem talking to the CP to fetch runner status: %v", err)
		return
	}

	seen := make(map[string]bool)
	for _, host := range latestHosts {
		_, ok := lbg.runners[host.Id]
		if ok {
			logrus.Debugf(" ... host %v at %d is known", host.Id, host.Address)

			// We already know about this
			seen[host.Id] = true
		} else {
			logrus.Infof(" ... host %v at %d is new", host.Id, host.Address)

			// This is a new runner. Bring it into the active pool
			runner := &runner{
				id:       host.Id,
				address:  host.Address,
				status:   RUNNER_ACTIVE,
				capacity: host.Capacity,
			}
			lbg.runners[host.Id] = runner
			lbg.active_runners = append(lbg.active_runners, runner)
			lbg.current_capacity += runner.capacity // The total capacity is already computed, since we asked for this
		}
	}

	// Work out if runners that we asked to be killed have been shut down
	logrus.Debugf("Removing dead hosts for %v", lbg.Id())
	// TODO the control plane might pull active or draining hosts out from under us. Deal with that too.
	dead := make([]*runner, 0)
	for _, runner := range lbg.dead_runners {
		if _, ok := seen[runner.id]; ok {
			// This runner is not yet shut down
			dead = append(dead, runner)
		} else {
			delete(lbg.runners, runner.id)
		}
	}
	lbg.dead_runners = dead
}
