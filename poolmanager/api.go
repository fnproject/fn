package poolmanager

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fnproject/fn/api/id"
	models "github.com/fnproject/fn/api/models"
	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/go-openapi/strfmt"
	"google.golang.org/grpc"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/hybrid"
)

type PoolManagerClient interface {
	AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error
	GetCall(req *http.Request) (*models.Call, error)
	GetGroupId(req *http.Request) (*model.LBGroupId, error)
	GetLBGroupMembership(id *model.LBGroupId) (*model.LBGroupMembership, error)
	Shutdown() error
}

type grpcPoolManagerClient struct {
	scaler  model.NodePoolScalerClient
	manager model.RunnerManagerClient
	agent   agent.DataAccess
	conn    *grpc.ClientConn
}

func NewPoolManagerClient(serverAddr string) (PoolManagerClient, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure()}
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	agent, err := hybrid.NewClient("localhost:8888")
	if err != nil {
		return nil, err
	}

	return &grpcPoolManagerClient{
		agent:   agent,
		scaler:  model.NewNodePoolScalerClient(conn),
		manager: model.NewRunnerManagerClient(conn),
		conn:    conn,
	}, nil
}

func (c *grpcPoolManagerClient) AdvertiseCapacity(snapshots *model.CapacitySnapshotList) error {
	_, err := c.scaler.AdvertiseCapacity(context.Background(), snapshots)
	if err != nil {
		log.Fatalf("Failed to push snapshots %v", err)
		return err
	}
	return nil
}

func (c *grpcPoolManagerClient) GetGroupId(req *http.Request) (*model.LBGroupId, error) {
	// TODO we need to make LBGroups part of data model
	return &model.LBGroupId{Id: "foobar"}, nil
}

func (c *grpcPoolManagerClient) GetCall(req *http.Request) (*models.Call, error) {
	// TODO we need to make this multitenant to globally resolve app and route from host domain
	// assuming single-tenant for now (hostname/r/app/route)

	tokens := strings.SplitN(req.URL.Path, "/", 4)
	app, err := c.agent.GetApp(context.Background(), tokens[2])
	if err != nil {
		log.Println("Failed to get app")
		return nil, err
	}

	route, err := c.agent.GetRoute(context.Background(), tokens[2], "/"+tokens[3])
	if err != nil {
		log.Println("Failed to get route")
		return nil, err
	}

	if route.Format == "" {
		route.Format = models.FormatDefault
	}

	id := id.New().String()

	// this ensures that there is an image, path, timeouts, memory, etc are valid.
	// NOTE: this means assign any changes above into route's fields
	err = route.Validate()
	if err != nil {
		return nil, err
	}

	return &models.Call{
		ID:      id,
		AppName: app.Name,
		Path:    route.Path,
		Image:   route.Image,
		// Delay: 0,
		Type:   route.Type,
		Format: route.Format,
		// Payload: TODO,
		Priority:    new(int32), // TODO this is crucial, apparently
		Timeout:     route.Timeout,
		IdleTimeout: route.IdleTimeout,
		Memory:      route.Memory,
		CPUs:        route.CPUs,
		Config:      buildConfig(app, route),
		Headers:     req.Header,
		CreatedAt:   strfmt.DateTime(time.Now()),
		URL:         reqURL(req),
		Method:      req.Method,
	}, nil

}

func buildConfig(app *models.App, route *models.Route) models.Config {
	conf := make(models.Config, 8+len(app.Config)+len(route.Config))
	for k, v := range app.Config {
		conf[k] = v
	}
	for k, v := range route.Config {
		conf[k] = v
	}

	conf["FN_FORMAT"] = route.Format
	conf["FN_APP_NAME"] = app.Name
	conf["FN_PATH"] = route.Path
	// TODO: might be a good idea to pass in: "FN_BASE_PATH" = fmt.Sprintf("/r/%s", appName) || "/" if using DNS entries per app
	conf["FN_MEMORY"] = fmt.Sprintf("%d", route.Memory)
	conf["FN_TYPE"] = route.Type

	CPUs := route.CPUs.String()
	if CPUs != "" {
		conf["FN_CPUS"] = CPUs
	}
	return conf
}

func reqURL(req *http.Request) string {
	if req.URL.Scheme == "" {
		if req.TLS == nil {
			req.URL.Scheme = "http"
		} else {
			req.URL.Scheme = "https"
		}
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	return req.URL.String()
}

func (c *grpcPoolManagerClient) GetLBGroupMembership(id *model.LBGroupId) (*model.LBGroupMembership, error) {
	return c.manager.GetLBGroup(context.Background(), id)
}

func (c *grpcPoolManagerClient) Shutdown() error {
	return c.conn.Close()
}
