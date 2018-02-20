package main

import (
	"context"
	"log"
	"net"

	google_protobuf1 "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/fnproject/fn/poolmanager/server/cp"
	"github.com/fnproject/fn/poolmanager"

)

type npmService struct {
	// Control plane "client"
	capMan   poolmanager.CapacityManager
}

func newNPMService(ctx context.Context, cp cp.ControlPlane) *npmService {
	return &npmService{
		capMan: poolmanager.NewCapacityManager(ctx, cp),
	}
}

func (npm *npmService) AdvertiseCapacity(ctx context.Context, snapshots *model.CapacitySnapshotList) (*google_protobuf1.Empty, error) {
	log.Printf("Received advertise capacity request %+v\n", snapshots)

	npm.capMan.Merge(snapshots)
	return nil, nil
}

func (npm *npmService) GetLBGroup(ctx context.Context, gid *model.LBGroupId) (*model.LBGroupMembership, error) {
	lbg := npm.capMan.LBGroup(gid.GetId())

	membership := &model.LBGroupMembership{GroupId: gid}
	runners := lbg.GetMembers()
	members := make([]*model.Runner, len(runners))
	for i, r := range runners {
		members[i] = &model.Runner{Address: r}
	}
	return membership, nil
}

func main() {

	gRPCServer := grpc.NewServer()

	log.Println("Starting Node Pool Manager gRPC service")

	svc := newNPMService(context.Background(), cp.NewControlPlane())
	model.RegisterNodePoolScalerServer(gRPCServer, svc)
	model.RegisterRunnerManagerServer(gRPCServer, svc)

	l, err := net.Listen("tcp", "0.0.0.0:8090")
	if err != nil {
		log.Panic("Failed to start server", err)
	}

	gRPCServer.Serve(l)
}
