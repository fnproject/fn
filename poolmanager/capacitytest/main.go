package main

import (
	model "github.com/fnproject/fn/poolmanager/grpc"
	ptypes "github.com/golang/protobuf/ptypes"
)

func main() {

	snapshots := make([]&model.CapacitySnapshot{})

	list := &model.CapacitySnapshotList{
		Ts:        ptypes.TimestampNow(),
		LbId:      "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		Snapshots: snapshots,
	}

}
