package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/drivers"
	rproto "github.com/fnproject/fn/api/agent/grpc"
	"github.com/fnproject/fn/api/agent/hybrid"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/api/server"
	_ "github.com/fnproject/fn/api/server/defaultexts"
	"github.com/gin-gonic/gin"

	// We need docker client here, since we have a custom driver that wraps generic
	// docker driver.
	docker "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	// TODO: Make these ports configurable, eg based on a provided range.
	LBPort     = 8081
	APIPort    = 8082
	LBAddress  = "http://127.0.0.1:8081"
	APIAddress = "http://127.0.0.1:8082"

	RunnerStartPort     = 8083
	RunnerStartGRPCPort = 9190

	StatusImage       = "fnproject/fn-status-checker:latest"
	StatusBarrierFile = "./barrier_file.txt"
)

var (
	viewKeys = []string{}
	viewDist = []float64{1, 10, 50, 100, 250, 500, 1000, 10000, 60000, 120000}
)

func LB() (string, error) {
	u, err := url.Parse(LBAddress)
	if err != nil {
		return "", err
	}
	return u.Host, nil
}

func CreateRunnerPool(runners []string) (pool.RunnerPool, error) {
	var dialOpts []grpc.DialOption

	//
	// Keepalive Client Side Settings (see: https://godoc.org/google.golang.org/grpc/keepalive#ClientParameters)
	//
	dialOpts = append(dialOpts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                time.Duration(60 * time.Second), // initiate a client initiated ping after 60 secs of inactivity
		Timeout:             time.Duration(10 * time.Second), // after a client initiated ping, the amount of time to wait for server to respond
		PermitWithoutStream: true,                            // send keepalive pings even if no active streams/RPCs
	}))

	return agent.NewStaticRunnerPool(runners, nil, dialOpts...), nil
}

// A runner pool with no docker networks available (using runner4)
func NewSystemTestNodePoolNoNet() (pool.RunnerPool, error) {
	runners := []string{
		"127.0.0.1:9193",
	}
	return CreateRunnerPool(runners)
}

func NewSystemTestNodePool() (pool.RunnerPool, error) {
	runners := []string{
		"127.0.0.1:9190",
		"127.0.0.1:9191",
		"127.0.0.1:9192",
	}
	return CreateRunnerPool(runners)
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

	pr0, err := SetUpPureRunnerNode(ctx, 0, "")
	if err != nil {
		return state, err
	}
	pr1, err := SetUpPureRunnerNode(ctx, 1, "")
	if err != nil {
		return state, err
	}
	pr2, err := SetUpPureRunnerNode(ctx, 2, "")
	if err != nil {
		return state, err
	}

	os.Remove(StatusBarrierFile)
	pr3, err := SetUpPureRunnerNode(ctx, 3, StatusBarrierFile)
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
	go func() { pr3.Start(ctx) }()
	logrus.Info("Started Pure Runner nodes")

	logrus.Info("Started Servers")
	// Wait for init - not great
	time.Sleep(5 * time.Second)
	return state, nil
}

func downloadMetrics() {

	time.Sleep(4 * time.Second)
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

	_, err := http.Get(LBAddress + "/shutdown")
	if err != nil {
		return err
	}
	_, err = http.Get(APIAddress + "/shutdown")
	if err != nil {
		return err
	}

	for i := 0; i < 4; i++ {
		_, err := http.Get(fmt.Sprintf("http://127.0.0.1:%v/shutdown", i+RunnerStartPort))
		if err != nil {
			return err
		}
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
	defaultDB := fmt.Sprintf("sqlite3://%s/data/fn.db", curDir)
	nodeType := server.ServerTypeAPI
	opts := make([]server.Option, 0)
	opts = append(opts, server.WithWebPort(APIPort))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogFormat(getEnv(server.EnvLogFormat, server.DefaultLogFormat)))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "API"))
	opts = append(opts, server.WithDBURL(getEnv(server.EnvDBURL, defaultDB)))
	opts = append(opts, server.WithTriggerAnnotator(server.NewStaticURLTriggerAnnotator(LBAddress)))
	opts = append(opts, server.WithFnAnnotator(server.NewStaticURLFnAnnotator(LBAddress)))
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly
	return server.New(ctx, opts...), nil
}

func SetUpLBNode(ctx context.Context) (*server.Server, error) {
	nodeType := server.ServerTypeLB
	opts := make([]server.Option, 0)
	opts = append(opts, server.WithWebPort(LBPort))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogFormat(getEnv(server.EnvLogFormat, server.DefaultLogFormat)))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "LB"))
	opts = append(opts, server.WithDBURL(""))
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly
	ridProvider := &server.RIDProvider{
		HeaderName:   "fn_request_id",
		RIDGenerator: common.FnRequestID,
	}
	opts = append(opts, server.WithRIDProvider(ridProvider))
	opts = append(opts, server.WithPrometheus())

	cl, err := hybrid.NewClient(APIAddress)
	if err != nil {
		return nil, err
	}
	nodePool, err := NewSystemTestNodePool()
	if err != nil {
		return nil, err
	}
	placerCfg := pool.NewPlacerConfig()
	placer := pool.NewNaivePlacer(&placerCfg)

	pool.RegisterPlacerViews(viewKeys, viewDist)
	agent.RegisterLBAgentViews(viewKeys, viewDist)
	agent.RegisterRunnerViews(viewKeys, viewDist) // yes, runner views via LB, we are a single process/exporter

	// Create an LB Agent with a Call Overrider to intercept calls in GetCall(). Overrider in this example
	// scrubs CPU/TmpFsSize and adds FN_CHEESE key/value into extensions.
	lbAgent, err := agent.NewLBAgent(nodePool, placer, agent.WithLBCallOverrider(LBCallOverrider))
	if err != nil {
		return nil, err
	}

	opts = append(opts, server.WithAgent(lbAgent), server.WithReadDataAccess(agent.NewCachedDataAccess(cl)))
	return server.New(ctx, opts...), nil
}

type logStream struct {
}

func (l *logStream) StreamLogs(logStream rproto.RunnerProtocol_StreamLogsServer) error {

	msg, err := logStream.Recv()
	if err != nil {
		return err
	}
	start := msg.GetStart()
	if start == nil {
		return errors.New("expected start session")
	}

	logrus.Infof("StreamLogs received start message %+v", start)

	for {
		msg, err := logStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		ack := msg.GetAck()
		if ack == nil {
			return errors.New("expected ack")
		}
		logrus.Infof("StreamLogs received ACK message %+v", ack)

		line := &rproto.LogResponseMsg_Container_Request_Line{
			Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
			Source:    rproto.LogResponseMsg_Container_Request_Line_STDOUT,
		}

		request := &rproto.LogResponseMsg_Container_Request{
			RequestId: "101",
			Data:      make([]*rproto.LogResponseMsg_Container_Request_Line, 0, 1),
		}
		request.Data = append(request.Data, line)

		container := &rproto.LogResponseMsg_Container{
			ApplicationId: "app1",
			FunctionId:    "fun1",
			ContainerId:   "container1",
			Data:          make([]*rproto.LogResponseMsg_Container_Request, 0, 1),
		}
		container.Data = append(container.Data, request)

		resp := &rproto.LogResponseMsg{
			Data: make([]*rproto.LogResponseMsg_Container, 0, 1),
		}
		resp.Data = append(resp.Data, container)

		logrus.Infof("StreamLogs sending Resp message %+v", resp)

		err = logStream.Send(resp)
		if err != nil {
			return err
		}
	}

	return nil
}

func SetUpPureRunnerNode(ctx context.Context, nodeNum int, StatusBarrierFile string) (*server.Server, error) {
	nodeType := server.ServerTypePureRunner
	opts := make([]server.Option, 0)
	opts = append(opts, server.WithWebPort(RunnerStartPort+nodeNum))
	opts = append(opts, server.WithGRPCPort(RunnerStartGRPCPort+nodeNum))
	opts = append(opts, server.WithType(nodeType))
	opts = append(opts, server.WithLogFormat(getEnv(server.EnvLogFormat, server.DefaultLogFormat)))
	opts = append(opts, server.WithLogLevel(getEnv(server.EnvLogLevel, server.DefaultLogLevel)))
	opts = append(opts, server.WithLogDest(getEnv(server.EnvLogDest, server.DefaultLogDest), "PURE-RUNNER"))
	opts = append(opts, server.WithDBURL(""))
	opts = append(opts, server.EnableShutdownEndpoint(ctx, func() {})) // TODO: do it properly

	grpcAddr := fmt.Sprintf(":%d", RunnerStartGRPCPort+nodeNum)

	// This is our Agent config, which we will use for both inner agent and docker.
	cfg, err := agent.NewConfig()
	if err != nil {
		return nil, err
	}

	cfg.ContainerLabelTag = fmt.Sprintf("fn-runner-%d", nodeNum)

	// customer driver that overrides generic docker driver
	d, err := agent.NewDockerDriver(cfg)
	if err != nil {
		return nil, err
	}
	drv := &customDriver{
		drv: d,
	}

	// inner agent for pure-runners
	innerAgent := agent.New(
		agent.WithConfig(cfg),
		agent.WithDockerDriver(drv),
		agent.WithCallOverrider(PureRunnerCallOverrider))

	cancelCtx, cancel := context.WithCancel(ctx)

	var grpcOpts []grpc.ServerOption

	//
	// Keepalive Server Side Settings (see: https://godoc.org/google.golang.org/grpc/keepalive#ServerParameters)
	//
	grpcOpts = append(grpcOpts, grpc.KeepaliveParams(keepalive.ServerParameters{
		Time:    time.Duration(60 * time.Second), // initiate a server initiated ping after 60 secs of inactivity
		Timeout: time.Duration(10 * time.Second), // after a server ping, the amount of time to wait for client to respond
	}))
	grpcOpts = append(grpcOpts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime:             time.Duration(5 * time.Second), // more frequent (client initiated) pings than every 5 secs is considered malicious
		PermitWithoutStream: true,                           // allow client pings even if no active stream
	}))

	var streamer logStream

	// now create pure-runner that wraps agent.
	pureRunner, err := agent.NewPureRunner(cancel, grpcAddr,
		agent.PureRunnerWithAgent(innerAgent),
		agent.PureRunnerWithStatusImage(StatusImage),
		agent.PureRunnerWithDetached(),
		agent.PureRunnerWithGRPCServerOptions(grpcOpts...),
		agent.PureRunnerWithStatusNetworkEnabler(StatusBarrierFile),
		agent.PureRunnerWithConfigFunc(configureRunner),
		agent.PureRunnerWithCustomHealthCheckerFunc(customHealthChecker),
		agent.PureRunnerWithLogStreamer(&streamer),
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
func LBCallOverrider(req *http.Request, c *models.Call, exts map[string]string) (map[string]string, error) {

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
func PureRunnerCallOverrider(req *http.Request, c *models.Call, exts map[string]string) (map[string]string, error) {

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
func (d *customDriver) SetPullImageRetryPolicy(policy common.BackOffConfig, checker drivers.RetryErrorChecker) error {
	return d.drv.SetPullImageRetryPolicy(policy, checker)
}

// implements Driver
func (d *customDriver) GetSlotKeyExtensions(extn map[string]string) string {
	return ""
}

// implements Driver
func (d *customDriver) Close() error {
	return d.drv.Close()
}

var _ drivers.Driver = &customDriver{}

// capture logs so they shut up when things are fine
func setLogBuffer() *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteByte('\n')
	logrus.SetOutput(&buf)
	gin.DefaultErrorWriter = &buf
	gin.DefaultWriter = &buf
	log.SetOutput(&buf)
	return &buf
}

var configureRunnerSetsThis map[string]string

func configureRunner(ctx context.Context, config *rproto.ConfigMsg) (*rproto.ConfigStatus, error) {
	if config.Config != nil {
		configureRunnerSetsThis = config.Config
	}
	return &rproto.ConfigStatus{}, nil
}

var shouldCustomHealthCheckerFail = false

func customHealthChecker(ctx context.Context) (map[string]string, error) {
	if !shouldCustomHealthCheckerFail {
		return map[string]string{
			"custom": "works",
		}, nil
	}

	return nil, models.NewAPIError(450, errors.New("Custom healthcheck failed"))
}

func runnerGrpcServerAddr(nodeNum int) string {
	targetHost := "127.0.0.1"
	if nodeNum <= 0 || nodeNum > 2 {
		nodeNum = rand.Intn(2)
	}
	return fmt.Sprintf("%s:%d", targetHost, RunnerStartGRPCPort+nodeNum)
}
