package poolmanager

import (
	"context"

	"github.com/fnproject/fn/grpcutil"
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

func newRemoteClient(serverAddr string, cert string, key string, ca string) (remoteClient, error) {
	logrus.WithField("npm_address", serverAddr).Info("Connecting to node pool manager")
	ctx := context.Background()
	creds, err := grpcutil.CreateCredentials(cert, key, ca)
	if err != nil {
		logrus.WithError(err).Error("Unable to create credentials to connect to runner node")
		return nil, err
	}

	conn, err := grpcutil.DialWithBackoff(ctx, serverAddr, creds, grpc.DefaultBackoffConfig)
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
		logrus.WithError(err).Warn("Failed to advertise capacity")
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
