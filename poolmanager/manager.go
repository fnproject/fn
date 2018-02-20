package poolmanager

import (
	"time"

	model "github.com/fnproject/fn/poolmanager/grpc"
)

type CapacityManager interface {
	Merge(*model.CapacitySnapshotList)
	Purge(time.Time, func(LBGroupId, LBId)) // Remove requirements that are too old to consider current
}

type LBGroupId string
type LBId string

type lbgRequirement struct {
	ts           time.Time // Time of last update
	in_use       int64
	total_wanted int64
}

type lbgCapacityRequirements struct {
	in_use       int64
	total_wanted int64
	requirements map[LBId]*lbgRequirement // NuLB id -> (ts, total_wanted)
}

type capacityManager struct {
	requirements map[LBGroupId]*lbgCapacityRequirements // LBGroup -> (totals, {lbid -> partials})
}

func NewCapacityManager() CapacityManager {
	return &capacityManager{}
}

func (m *capacityManager) Merge(list *model.CapacitySnapshotList) {
	lbid := LBId(list.GetLbId())
	for _, new_req := range list.Snapshots {
		lbg := LBGroupId(new_req.GetGroupId().GetId())
		currentReqs, ok := m.requirements[lbg]
		if !ok {
			// We have a new LBGroup we don't know about, insert and update
			currentReqs = &lbgCapacityRequirements{}
			m.requirements[lbg] = currentReqs
		}

		// Add in the new requirements, removing the old ones if required.
		old_req, ok := currentReqs.requirements[lbid]
		if !ok {
			// This is a new LB that we're just learning about
			old_req = &lbgRequirement{}
			currentReqs.requirements[lbid] = old_req
		}

		// Update totals: remove this LB's previous capacity assertions
		currentReqs.in_use -= old_req.in_use
		currentReqs.total_wanted -= old_req.total_wanted

		// Update totals: add this LB's new assertions and record them
		currentReqs.total_wanted += int64(new_req.GetMemMbTotal())

		// Keep a copy of this requirement
		old_req.ts = time.Now()
		old_req.total_wanted = int64(new_req.GetMemMbTotal())

		// TODO: new_req also has a generation for the runner information that LB held. If that's out of date, signal that we need to readvertise
	}
}

func (m *capacityManager) Purge(oldest time.Time, cb func(LBGroupId, LBId)) {
	// For the moment, just strip out old details.
	// In future, we should do something about these LBs that aren't talking to us. That's what the cb is for.
	for lbg, reqs := range m.requirements {
		for lbid, req := range reqs.requirements {
			if req.ts.Before(oldest) {
				// We need to nix this entry, it's utterly out-of-date
				reqs.in_use -= req.in_use
				reqs.total_wanted -= req.total_wanted
				delete(reqs.requirements, lbid)

				// TODO: use a callback here to handle the deletion?
				cb(lbg, lbid)
			}
		}
	}
}
