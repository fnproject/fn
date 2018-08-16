package tests

import (
	"bytes"
	"context"
	"fmt"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/drivers"
	"github.com/fnproject/fn/api/agent/hybrid"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/api/server"
	_ "github.com/fnproject/fn/api/server/defaultexts"

	// We need docker client here, since we have a custom driver that wraps generic
	// docker driver.
	"github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"

	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	LBAddress   = "http://127.0.0.1:8081"
	StatusImage = "fnproject/fn-status-checker:latest"
)

func LB() (string, error) {
	u, err := url.Parse(LBAddress)
	if err != nil {
		return "", err
	}
	return u.Host, nil
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

func setUpSystem() (*state, error) {
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

func downloadMetrics() {

	fileName, ok := os.LookupEnv("SYSTEM_TEST_PROMETHEUS_FILE")
	if !ok || fileName == "" {
		return
	}

	resp, err := http.Get(LBAddress + "/metrics")
	if err != nil {
		logrus.WithError(err).Fatal("Fetching metrics, got unexpected error")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Fatal("Reading metrics body, got unexpected error")
	}

	err = ioutil.WriteFile(fileName, body, 0644)
	if err != nil {
		logrus.WithError(err).Fatalf("Writing metrics body to %v, got unexpected error", fileName)
	}
}

func CleanUpSystem(st *state) error {

	downloadMetrics()

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
	opts := make([]server.Option, 0)
	opts = append(opts, server.WithWebPort(8085))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "API"))
	opts = append(opts, server.WithDBURL(getEnv(server.EnvDBURL, defaultDB)))
	opts = append(opts, server.WithMQURL(getEnv(server.EnvMQURL, defaultMQ)))
	opts = append(opts, server.WithLogURL(""))
	opts = append(opts, server.WithLogstoreFromDatastore())
	opts = append(opts, server.WithTriggerAnnotator(server.NewStaticURLTriggerAnnotator("http://localhost:8081")))
	opts = append(opts, server.WithFnAnnotator(server.NewStaticURLFnAnnotator("http://localhost:8081")))
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly
	return server.New(ctx, opts...), nil
}

func SetUpLBNode(ctx context.Context) (*server.Server, error) {
	nodeType := server.ServerTypeLB
	opts := make([]server.Option, 0)
	opts = append(opts, server.WithWebPort(8081))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "LB"))
	opts = append(opts, server.WithDBURL(""))
	opts = append(opts, server.WithMQURL(""))
	opts = append(opts, server.WithLogURL(""))
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly
	ridProvider := &server.RIDProvider{
		HeaderName:   "fn_request_id",
		RIDGenerator: common.FnRequestID,
	}
	opts = append(opts, server.WithRIDProvider(ridProvider))
	opts = append(opts, server.WithPrometheus())

	apiURL := "http://127.0.0.1:8085"
	cl, err := hybrid.NewClient(apiURL)
	if err != nil {
		return nil, err
	}
	nodePool, err := NewSystemTestNodePool()
	if err != nil {
		return nil, err
	}
	placerCfg := pool.NewPlacerConfig()
	placer := pool.NewNaivePlacer(&placerCfg)

	keys := []string{"fn_appname", "fn_path"}
	dist := []float64{1, 10, 50, 100, 250, 500, 1000, 10000, 60000, 120000}
	pool.RegisterPlacerViews(keys, dist)
	agent.RegisterLBAgentViews(keys, dist)

	// Create an LB Agent with a Call Overrider to intercept calls in GetCall(). Overrider in this example
	// scrubs CPU/TmpFsSize and adds FN_CHEESE key/value into extensions.
	lbAgent, err := agent.NewLBAgent(cl, nodePool, placer, agent.WithLBCallOverrider(LBCallOverrider))
	if err != nil {
		return nil, err
	}

	opts = append(opts, server.WithAgent(lbAgent), server.WithReadDataAccess(agent.NewCachedDataAccess(cl)))
	return server.New(ctx, opts...), nil
}

func SetUpPureRunnerNode(ctx context.Context, nodeNum int) (*server.Server, error) {
	nodeType := server.ServerTypePureRunner
	opts := make([]server.Option, 0)
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

	// This is our Agent config, which we will use for both inner agent and docker.
	cfg, err := agent.NewConfig()
	if err != nil {
		return nil, err
	}

	// customer driver that overrides generic docker driver
	d, err := agent.NewDockerDriver(cfg)
	if err != nil {
		return nil, err
	}
	drv := &customDriver{
		drv: d,
	}

	// inner agent for pure-runners
	innerAgent := agent.New(ds,
		agent.WithConfig(cfg),
		agent.WithDockerDriver(drv),
		agent.WithCallOverrider(PureRunnerCallOverrider))

	cancelCtx, cancel := context.WithCancel(ctx)

	// now create pure-runner that wraps agent.
	pureRunner, err := agent.NewPureRunner(cancel, grpcAddr,
		agent.PureRunnerWithAgent(innerAgent),
		agent.PureRunnerWithStatusImage(StatusImage),
	)
	if err != nil {
		return nil, err
	}

	opts = append(opts, server.WithAgent(pureRunner), server.WithExtraCtx(cancelCtx))
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
		if i.Name == "docker0" || i.Name == "vboxnet0" || i.Name == "lo" {
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
	state, err := setUpSystem()
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

// Memory Only LB Agent Call Option
func LBCallOverrider(c *models.Call, exts map[string]string) (map[string]string, error) {

	// Set TmpFsSize and CPU to unlimited. This means LB operates on Memory
	// only. Operators/Service providers are expected to override this
	// and apply their own filter to set/override CPU/TmpFsSize/Memory
	// and Extension variables.
	c.TmpFsSize = 0
	c.CPUs = models.MilliCPUs(0)
	delete(c.Config, "FN_CPUS")

	if exts == nil {
		exts = make(map[string]string)
	}

	// Add an FN_CHEESE extension to be intercepted and specially handled by Pure Runner customDriver below
	exts["FN_CHEESE"] = "Tete de Moine"
	return exts, nil
}

// Pure Runner Agent Call Option
func PureRunnerCallOverrider(c *models.Call, exts map[string]string) (map[string]string, error) {

	if exts == nil {
		exts = make(map[string]string)
	}

	// Add an FN_WINE extension, just an example...
	exts["FN_WINE"] = "1982 Margaux"
	return exts, nil
}

// An example Pure Runner docker driver. Using CreateCookie, it intercepts a generated cookie to
// add an environment variable FN_CHEESE if it finds a FN_CHEESE extension.
type customDriver struct {
	drv drivers.Driver
}

// implements Driver
func (d *customDriver) CreateCookie(ctx context.Context, task drivers.ContainerTask) (drivers.Cookie, error) {
	cookie, err := d.drv.CreateCookie(ctx, task)
	if err != nil {
		return cookie, err
	}

	// docker driver specific data
	obj := cookie.ContainerOptions()
	opts, ok := obj.(docker.CreateContainerOptions)
	if !ok {
		logrus.Fatal("Unexpected driver, should be docker")
	}

	// if call extensions include 'foo', then let's add FN_CHEESE env vars, which should
	// end up in Env/Config.
	ext := task.Extensions()
	cheese, ok := ext["FN_CHEESE"]
	if ok {
		opts.Config.Env = append(opts.Config.Env, "FN_CHEESE="+cheese)
	}

	wine, ok := ext["FN_WINE"]
	if ok {
		opts.Config.Env = append(opts.Config.Env, "FN_WINE="+wine)
	}

	return cookie, nil
}

// implements Driver
func (d *customDriver) PrepareCookie(ctx context.Context, cookie drivers.Cookie) error {
	return d.drv.PrepareCookie(ctx, cookie)
}

// implements Driver
func (d *customDriver) Close() error {
	return d.drv.Close()
}

var _ drivers.Driver = &customDriver{}
