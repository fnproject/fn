package poolmanager

import (
	model "github.com/fnproject/fn/poolmanager/grpc"
)

type CapacityManager interface {
	Merge(*model.CapacitySnapshotList)
}

type capacityManager struct {
}
