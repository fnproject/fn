package main

import (
	"context"
	"net"

	google_protobuf1 "github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/fnproject/fn/poolmanager"
	model "github.com/fnproject/fn/poolmanager/grpc"
	"github.com/fnproject/fn/poolmanager/server/cp"

	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

type npmService struct {
	// Control plane "client"
	capMan poolmanager.CapacityManager
}

func newNPMService(ctx context.Context, cp cp.ControlPlane) *npmService {
	cm, err := poolmanager.NewCapacityManager(ctx, cp)
	if err != nil {
		logrus.Panic("Cannot construct capacity manager")
	}
	return &npmService{
		capMan: cm,
	}
}

func (npm *npmService) AdvertiseCapacity(ctx context.Context, snapshots *model.CapacitySnapshotList) (*google_protobuf1.Empty, error) {
	npm.capMan.Merge(snapshots)
	logrus.Debugf("Merged capacity requests %+v", snapshots)
	return &google_protobuf1.Empty{}, nil
}

func (npm *npmService) GetLBGroup(ctx context.Context, gid *model.LBGroupId) (*model.LBGroupMembership, error) {
	lbg := npm.capMan.LBGroup(gid.GetId())

	membership := &model.LBGroupMembership{GroupId: gid}
	members := lbg.GetMembers()
	runners := make([]*model.Runner, len(members))
	for i, r := range members {
		runners[i] = &model.Runner{Address: r}
	}
	membership.Runners = runners
	if len(runners) > 0 {
		logrus.Infof("LBGroup membership for %v is %+v", gid.GetId(), runners)
	} else {
		logrus.Debugf("LBGroup membership for %v is %+v", gid.GetId(), runners)
	}
	return membership, nil
}

const (
	// Certificates to communicate with other FN nodes
	EnvCert         = "FN_NODE_CERT"
	EnvCertKey      = "FN_NODE_CERT_KEY"
	EnvCertAuth     = "FN_NODE_CERT_AUTHORITY"
	EnvPort         = "FN_PORT"
	EnvFixedRunners = "FN_RUNNER_ADDRESSES"
)

func getAndCheckFile(envVar string) (string, error) {
	filename := getEnv(envVar)
	if filename == "" {
		return "", fmt.Errorf("Please provide a valid file path in the %v variable", envVar)
	}
	abs, err := filepath.Abs(filename)
	if err != nil {
		return "", fmt.Errorf("Unable to resolve %v: please specify a valid and readable file", filename)
	}
	_, err = os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("Cannot stat %v: please specify a valid and readable file", abs)
	}
	return abs, nil
}

func createGrpcCreds(cert string, key string, ca string) (grpc.ServerOption, error) {
	// Load the certificates from disk
	certificate, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("could not load server key pair: %s", err)
	}

	// Create a certificate pool from the certificate authority
	certPool := x509.NewCertPool()
	authority, err := ioutil.ReadFile(ca)
	if err != nil {
		return nil, fmt.Errorf("could not read ca certificate: %s", err)
	}

	if ok := certPool.AppendCertsFromPEM(authority); !ok {
		return nil, errors.New("failed to append client certs")
	}

	creds := credentials.NewTLS(&tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	})

	return grpc.Creds(creds), nil
}

func main() {
	level, err := logrus.ParseLevel(getEnv("FN_LOG_LEVEL"))
	if err != nil {
		logrus.Panic("Set a valid FN_LOG_LEVEL")
	}
	logrus.SetLevel(level)

	// Obtain certificate paths
	cert, err := getAndCheckFile(EnvCert)
	if err != nil {
		logrus.Fatal(err)
	}
	key, err := getAndCheckFile(EnvCertKey)
	if err != nil {
		logrus.Fatal(err)
	}
	ca, err := getAndCheckFile(EnvCertAuth)
	if err != nil {
		logrus.Fatal(err)
	}

	gRPCCreds, err := createGrpcCreds(cert, key, ca)
	if err != nil {
		logrus.Fatal(err)
	}

	gRPCServer := grpc.NewServer(gRPCCreds)

	logrus.Info("Starting Node Pool Manager gRPC service")

	fakeRunners := strings.Split(getEnv(EnvFixedRunners), ",")
	svc := newNPMService(context.Background(), cp.NewControlPlane(fakeRunners))
	model.RegisterNodePoolScalerServer(gRPCServer, svc)
	model.RegisterRunnerManagerServer(gRPCServer, svc)

	port := getEnv(EnvPort)
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		logrus.Fatalf("could not listen on port %s: %s", port, err)
	}

	if err := gRPCServer.Serve(l); err != nil {
		logrus.Fatalf("grpc serve error: %s", err)
	}
}

func getEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Panicf("Missing config key: %v", key)
	}
	return value
}
