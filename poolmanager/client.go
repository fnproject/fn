package poolmanager

import (
	"context"

	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type remoteClient interface {
	AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error
	GetLBGroup(id *model.LBGroupId) (*model.LBGroupMembership, error)
	Shutdown() error
}

type grpcPoolManagerClient struct {
	scaler  model.NodePoolScalerClient
	manager model.RunnerManagerClient
	conn    *grpc.ClientConn
}

func newRemoteClient(serverAddr string) (remoteClient, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure()}
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return &grpcPoolManagerClient{
		scaler:  model.NewNodePoolScalerClient(conn),
		manager: model.NewRunnerManagerClient(conn),
		conn:    conn,
	}, nil
}

func (c *grpcPoolManagerClient) AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error {
	_, err := c.scaler.AdvertiseCapacity(context.Background(), snapshots)
	if err != nil {
		logrus.WithError(err).Error("Failed to push snapshots")
		return err
	}
	return nil
}

func (c *grpcPoolManagerClient) GetLBGroup(id *model.LBGroupId) (*model.LBGroupMembership, error) {
	return c.manager.GetLBGroup(context.Background(), id)
}

func (c *grpcPoolManagerClient) Shutdown() error {
	return c.conn.Close()
}
