package tests

import (
	"bytes"
	"context"
	"fmt"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/hybrid"
	"github.com/fnproject/fn/api/common"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/api/server"

	"github.com/sirupsen/logrus"

	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type SystemTestNodePool struct {
	runners []pool.Runner
}

func NewSystemTestNodePool() (pool.RunnerPool, error) {
	myAddr := whoAmI()
	runners := []string{
		fmt.Sprintf("%s:9190", myAddr),
		fmt.Sprintf("%s:9191", myAddr),
		fmt.Sprintf("%s:9192", myAddr),
	}
	return agent.DefaultStaticRunnerPool(runners), nil
}

type state struct {
	memory string
	cancel func()
}

func SetUpSystem() (*state, error) {
	ctx, cancel := context.WithCancel(context.Background())
	state := &state{
		cancel: cancel,
	}

	api, err := SetUpAPINode(ctx)
	if err != nil {
		return state, err
	}
	logrus.Info("Created API node")

	lb, err := SetUpLBNode(ctx)
	if err != nil {
		return state, err
	}
	logrus.Info("Created LB node")

	state.memory = os.Getenv(agent.EnvMaxTotalMemory)
	os.Setenv(agent.EnvMaxTotalMemory, strconv.FormatUint(256*1024*1024, 10))

	pr0, err := SetUpPureRunnerNode(ctx, 0)
	if err != nil {
		return state, err
	}
	pr1, err := SetUpPureRunnerNode(ctx, 1)
	if err != nil {
		return state, err
	}
	pr2, err := SetUpPureRunnerNode(ctx, 2)
	if err != nil {
		return state, err
	}
	logrus.Info("Created Pure Runner nodes")

	go func() { api.Start(ctx) }()
	logrus.Info("Started API node")
	go func() { lb.Start(ctx) }()
	logrus.Info("Started LB node")
	go func() { pr0.Start(ctx) }()
	go func() { pr1.Start(ctx) }()
	go func() { pr2.Start(ctx) }()
	logrus.Info("Started Pure Runner nodes")
	// Wait for init - not great
	time.Sleep(5 * time.Second)
	return state, nil
}

func CleanUpSystem(st *state) error {
	_, err := http.Get("http://127.0.0.1:8081/shutdown")
	if err != nil {
		return err
	}
	_, err = http.Get("http://127.0.0.1:8082/shutdown")
	if err != nil {
		return err
	}
	_, err = http.Get("http://127.0.0.1:8083/shutdown")
	if err != nil {
		return err
	}
	_, err = http.Get("http://127.0.0.1:8084/shutdown")
	if err != nil {
		return err
	}
	_, err = http.Get("http://127.0.0.1:8085/shutdown")
	if err != nil {
		return err
	}

	if st.cancel != nil {
		st.cancel()
	}

	// Wait for shutdown - not great
	time.Sleep(5 * time.Second)

	if st.memory != "" {
		os.Setenv(agent.EnvMaxTotalMemory, st.memory)
	} else {
		os.Unsetenv(agent.EnvMaxTotalMemory)
	}

	return nil
}

func SetUpAPINode(ctx context.Context) (*server.Server, error) {
	curDir := pwd()
	var defaultDB, defaultMQ string
	defaultDB = fmt.Sprintf("sqlite3://%s/data/fn.db", curDir)
	defaultMQ = fmt.Sprintf("bolt://%s/data/fn.mq", curDir)
	nodeType := server.ServerTypeAPI
	opts := make([]server.ServerOption, 0)
	opts = append(opts, server.WithWebPort(8085))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "API"))
	opts = append(opts, server.WithDBURL(getEnv(server.EnvDBURL, defaultDB)))
	opts = append(opts, server.WithMQURL(getEnv(server.EnvMQURL, defaultMQ)))
	opts = append(opts, server.WithLogURL(""))
	opts = append(opts, server.WithLogstoreFromDatastore())
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly
	return server.New(ctx, opts...), nil
}

func SetUpLBNode(ctx context.Context) (*server.Server, error) {
	nodeType := server.ServerTypeLB
	opts := make([]server.ServerOption, 0)
	opts = append(opts, server.WithWebPort(8081))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "LB"))
	opts = append(opts, server.WithDBURL(""))
	opts = append(opts, server.WithMQURL(""))
	opts = append(opts, server.WithLogURL(""))
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly
	ridProvider := &server.RIDProvider{
		HeaderName:   "fn-request-id",
		RIDGenerator: common.FnRequestID,
	}
	opts = append(opts, server.WithRIDProvider(ridProvider))

	apiURL := "http://127.0.0.1:8085"
	cl, err := hybrid.NewClient(apiURL)
	if err != nil {
		return nil, err
	}
	nodePool, err := NewSystemTestNodePool()
	if err != nil {
		return nil, err
	}
	placer := pool.NewNaivePlacer()
	agent, err := agent.NewLBAgent(agent.NewCachedDataAccess(cl), nodePool, placer)
	if err != nil {
		return nil, err
	}
	opts = append(opts, server.WithAgent(agent))

	return server.New(ctx, opts...), nil
}

func SetUpPureRunnerNode(ctx context.Context, nodeNum int) (*server.Server, error) {
	nodeType := server.ServerTypePureRunner
	opts := make([]server.ServerOption, 0)
	opts = append(opts, server.WithWebPort(8082+nodeNum))
	opts = append(opts, server.WithGRPCPort(9190+nodeNum))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "PURE-RUNNER"))
	opts = append(opts, server.WithDBURL(""))
	opts = append(opts, server.WithMQURL(""))
	opts = append(opts, server.WithLogURL(""))
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly

	ds, err := hybrid.NewNopDataStore()
	if err != nil {
		return nil, err
	}
	grpcAddr := fmt.Sprintf(":%d", 9190+nodeNum)
	cancelCtx, cancel := context.WithCancel(ctx)

	prAgent, err := agent.NewPureRunner(cancel, grpcAddr, ds, "", "", "", nil)
	if err != nil {
		return nil, err
	}
	opts = append(opts, server.WithAgent(prAgent), server.WithExtraCtx(cancelCtx))

	return server.New(ctx, opts...), nil
}

func pwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't get working directory, possibly unsupported platform?")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	return strings.Replace(cwd, "\\", "/", -1)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		// linter liked this better than if/else
		var err error
		var i int
		if i, err = strconv.Atoi(value); err != nil {
			panic(err) // not sure how to handle this
		}
		return i
	}
	return fallback
}

// whoAmI searches for a non-local address on any network interface, returning
// the first one it finds. it could be expanded to search eth0 or en0 only but
// to date this has been unnecessary.
func whoAmI() net.IP {
	ints, _ := net.Interfaces()
	for _, i := range ints {
		if i.Name == "docker0" || i.Name == "lo" {
			// not perfect
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			ip, _, err := net.ParseCIDR(a.String())
			if a.Network() == "ip+net" && err == nil && ip.To4() != nil {
				if !bytes.Equal(ip, net.ParseIP("127.0.0.1")) {
					return ip
				}
			}
		}
	}
	return nil
}

func TestMain(m *testing.M) {
	state, err := SetUpSystem()
	if err != nil {
		logrus.WithError(err).Fatal("Could not initialize system")
		os.Exit(1)
	}
	// call flag.Parse() here if TestMain uses flags
	result := m.Run()
	err = CleanUpSystem(state)
	if err != nil {
		logrus.WithError(err).Warn("Could not clean up system")
	}
	if result == 0 {
		fmt.Fprintln(os.Stdout, "😀  👍  🎗")
	}
	os.Exit(result)
}
