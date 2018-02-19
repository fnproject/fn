package main

import (
	"context"
	"log"
	"net"
	"sync"

	google_protobuf1 "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	model "github.com/fnproject/fn/poolmanager/grpc"
)

type npmService struct {
	lbgMtx   *sync.RWMutex
	lbGroups map[string][]string // lbg -> [runners]
}

func newNPMService() *npmService {
	return &npmService{lbgMtx: &sync.RWMutex{}, lbGroups: make(map[string][]string)}
}

func (npm *npmService) AdvertiseCapacity(ctx context.Context, snapshots *model.CapacitySnapshotList) (*google_protobuf1.Empty, error) {
	log.Printf("Received advertise capacity request %+v\n", snapshots)

	npm.lbgMtx.Lock()
	defer npm.lbgMtx.Unlock()

	for _, snapshot := range snapshots.GetSnapshots() {
		// look up and map
		if runners, ok := npm.lbGroups[snapshot.GetGroupId().GetId()]; !ok {
			if snapshot.GetMemMbTotal() > 0 {
				npm.lbGroups[snapshot.GetGroupId().GetId()] = []string{createInstance()}
			}
		}
	}
	return nil, nil
}

func (npm *npmService) GetLBGroup(ctx context.Context, gid *model.LBGroupId) (*model.LBGroupMembership, error) {
	npm.lbgMtx.RLock()
	defer npm.lbgMtx.RUnlock()

	membership := &model.LBGroupMembership{GroupId: gid}
	if runners, ok := npm.lbGroups[gid.GetId()]; ok {
		members := make([]*model.Runner, len(runners))
		for i, r := range runners {
			members[i] = &model.Runner{Address: r}
		}
	}
	return membership, nil
}

func createInstance() string {
	return "localhost:8888"
}

func main() {

	gRPCServer := grpc.NewServer()

	log.Println("Starting Node Pool Manager gRPC service")

	svc := newNPMService()
	model.RegisterNodePoolScalerServer(gRPCServer, svc)
	model.RegisterRunnerManagerServer(gRPCServer, svc)

	l, err := net.Listen("tcp", "8080")
	if err != nil {
		log.Panic("Failed to start server")
	}

	gRPCServer.Serve(l)
}
