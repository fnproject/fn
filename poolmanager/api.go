package poolmanager

import (
	"context"
	"log"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"google.golang.org/grpc"
)

func SendCapacitySnapshots(snapshots *model.CapacitySnapshotList) error {
	opts := []grpc.DialOption{
		grpc.WithInsecure()}
	conn, err := grpc.Dial(*serverAddr, opts)
	if err != nil {
		return err
	}
	defer conn.Close()
	client := model.NewNodePoolManagerClient(conn)
	stream, err := client.AdvertiseCapacity(context.Background())
	if err != nil {
		log.Fatalf("Failed to create snapshot stream %v", err)
		return err
	}
	if err := stream.Send(snapshots); err != nil {
		log.Fatalf("Failed to send snapshot %v", err)
		return err
	}

	_, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Failed to close snapshot stream %v", err)
		return err
	}
	return nil
}
