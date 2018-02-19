package main

import (
	"context"
	"log"
	"net"

	google_protobuf1 "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	model "github.com/fnproject/fn/poolmanager/grpc"
)

type npmService struct {
}

func (npm *npmService) AdvertiseCapacity(ctx context.Context, snapshots *model.CapacitySnapshotList) (*google_protobuf1.Empty, error) {
	log.Printf("Received advertise capacity request %+v\n", snapshots)
	return nil, nil
}

func main() {

	gRPCServer := grpc.NewServer()

	log.Println("Starting Node Pool Manager gRPC service")

	model.RegisterNodePoolScalerServer(gRPCServer, &npmService{})

	l, err := net.Listen("tcp", "8080")
	if err != nil {
		log.Panic("Failed to start server")
	}

	gRPCServer.Serve(l)
}
