package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/hybrid"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/models"
	pool "github.com/fnproject/fn/api/runnerpool"
	"github.com/fnproject/fn/api/version"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/exporter/zipkin"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

const (
	// TODO these are kind of redundant as exported values since the env vars
	// have to be set using these values (hopefully from an env), consider
	// forcing usage through WithXxx configuration methods and documenting there vs.
	// expecting users to use os.SetEnv(EnvLogLevel, "debug") // why ?

	// EnvLogFormat sets the stderr logging format, text or json only
	EnvLogFormat = "FN_LOG_FORMAT"

	// EnvLogLevel sets the stderr logging level
	EnvLogLevel = "FN_LOG_LEVEL"

	// EnvLogDest is a url of a log destination:
	// possible schemes: { udp, tcp, file }
	// file url must contain only a path, syslog must contain only a host[:port]
	// expect: [scheme://][host][:port][/path]
	// default scheme to udp:// if none given
	EnvLogDest = "FN_LOG_DEST"

	// EnvLogPrefix is a prefix to affix to each log line.
	EnvLogPrefix = "FN_LOG_PREFIX"

	// EnvDBURL is a url to a db service:
	// possible schemes: { postgres, sqlite3, mysql }
	EnvDBURL = "FN_DB_URL"

	// EnvRunnerURL is a url pointing to an Fn API service.
	EnvRunnerURL = "FN_RUNNER_API_URL"

	// EnvRunnerAddresses is a list of runner urls for an lb to use.
	EnvRunnerAddresses = "FN_RUNNER_ADDRESSES"

	// EnvPublicLoadBalancerURL is the url to inject into trigger responses to get a public url.
	EnvPublicLoadBalancerURL = "FN_PUBLIC_LB_URL"

	// EnvNodeType defines the runtime mode for fn to run in, options
	// are one of: { full, api, lb, runner, pure-runner }
	EnvNodeType = "FN_NODE_TYPE"

	// EnvPort is the port to listen on for fn http server.
	EnvPort = "FN_PORT" // be careful, Gin expects this variable to be "port"

	// EnvGRPCPort is the port to run the grpc server on for a pure-runner node.
	EnvGRPCPort = "FN_GRPC_PORT"

	// EnvAPICORSOrigins is the list of CORS origins to allow.
	EnvAPICORSOrigins = "FN_API_CORS_ORIGINS"

	// EnvAPICORSHeaders is the list of CORS headers allowed.
	EnvAPICORSHeaders = "FN_API_CORS_HEADERS"

	// EnvZipkinURL is the url of a zipkin node to send traces to.
	EnvZipkinURL = "FN_ZIPKIN_URL"

	// EnvJaegerURL is the url of a jaeger node to send traces to.
	EnvJaegerURL = "FN_JAEGER_URL"

	// EnvRIDHeader is the header name of the incoming request which holds the request ID
	EnvRIDHeader = "FN_RID_HEADER"

	// EnvProcessCollectorList is the list of procid's to collect metrics for.
	EnvProcessCollectorList = "FN_PROCESS_COLLECTOR_LIST"

	// EnvLBPlacementAlg is the algorithm to place fn calls to fn runners in lb.[0w
	EnvLBPlacementAlg = "FN_PLACER"

	// EnvMaxRequestSize sets the limit in bytes for any API request body's length.
	EnvMaxRequestSize = "FN_MAX_REQUEST_SIZE"

	// EnvMaxHeaderSize sets the limit in bytes for any API request body's length.
	EnvMaxHeaderSize = "FN_MAX_REQUEST_HEADER_SIZE"

	// The following 4 env-vars (FN_REQUEST_BODY_READ_TIMEOUT, FN_REQUEST_HEADER_READ_TIMEOUT, FN_RESPONSE_WRITE_TIMEOUT, FN_HTTP_IDLE_TIMEOUT)
	// need to be set as strings that are either :
	// 1. Valid integral values of duration in seconds ("120", "125")
	// 2. Valid duration-format strings ("120s", "2m5s")

	// EnvReadTimeout sets the timeout limit for reading a http request body.
	EnvReadTimeout = "FN_REQUEST_BODY_READ_TIMEOUT"

	// EnvReadHeaderTimeout sets the timeout limit for reading a http request header.
	EnvReadHeaderTimeout = "FN_REQUEST_HEADER_READ_TIMEOUT"

	// EnvWriteTimeout sets the timeout limit for writing a http response.
	EnvWriteTimeout = "FN_RESPONSE_WRITE_TIMEOUT"

	// EnvHTTPIdleTimeout maximum amount of time to wait for the next request.
	EnvHTTPIdleTimeout = "FN_HTTP_IDLE_TIMEOUT"

	// DefaultLogFormat is text
	DefaultLogFormat = "text"

	// DefaultLogLevel is info
	DefaultLogLevel = "info"

	// DefaultLogDest is stderr
	DefaultLogDest = "stderr"

	// DefaultPort is 8080
	DefaultPort = 8080

	// DefaultGRPCPort is 9190
	DefaultGRPCPort = 9190
)

// NodeType is the mode to run fn in.
type NodeType int32

const (
	// ServerTypeFull runs all API endpoints, including executing tasks.
	ServerTypeFull NodeType = iota

	// ServerTypeAPI runs only control plane endpoints, to manage resources.
	ServerTypeAPI

	// ServerTypeLB runs only invoke/http trigger endpoints, routing to runner nodes.
	ServerTypeLB

	// ServerTypePureRunner runs only grpc server, to execute tasks.
	ServerTypePureRunner
)

// Configuration keys to identify grpc, admin, web services:
const (
	// For backwards compat, TLS-prefix configuration-keys:
	TLSGRPCServer  = "gRPCServer"
	TLSAdminServer = "AdminServer"
	TLSWebServer   = "WebServer"

	AdminServer = "AdminServer"
	WebServer   = "WebServer"
	GRPCServer  = "gRPCServer"
)

func (s NodeType) String() string {
	switch s {
	case ServerTypeFull:
		return "full"
	case ServerTypeAPI:
		return "api"
	case ServerTypeLB:
		return "lb"
	case ServerTypePureRunner:
		return "pure-runner"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

// Server is the object which ties together all the fn things, it is the entrypoint
// for managing fn resources and executing tasks.
type Server struct {
	Router      *gin.Engine
	AdminRouter *gin.Engine

	agent     agent.Agent
	datastore models.Datastore
	nodeType  NodeType

	// Service Settings for Admin/Web/gRPC. Note that for gRPC only
	// TLSConfig and Addr are transferrable from http.Server to GRPC service.
	// TODO: extend this to cover gRPC options.
	svcConfigs map[string]*http.Server

	// Agent enqueue  and read stores
	lbReadAccess           agent.ReadDataAccess
	noHTTTPTriggerEndpoint bool
	noFnInvokeEndpoint     bool
	appListeners           *appListeners
	fnListeners            *fnListeners
	triggerListeners       *triggerListeners
	rootMiddlewares        []fnext.Middleware
	apiMiddlewares         []fnext.Middleware
	promExporter           *prometheus.Exporter
	triggerAnnotator       TriggerAnnotator
	fnAnnotator            FnAnnotator

	// Extensions can append to this list of contexts so that cancellations are properly handled.
	extraCtxs []context.Context
}

func nodeTypeFromString(value string) NodeType {
	switch value {
	case "api":
		return ServerTypeAPI
	case "lb":
		return ServerTypeLB
	case "pure-runner":
		return ServerTypePureRunner
	default:
		return ServerTypeFull
	}
}

// NewFromEnv creates a new Functions server based on env vars.
func NewFromEnv(ctx context.Context, opts ...Option) *Server {
	curDir := pwd()
	var defaultDB string
	nodeType := nodeTypeFromString(getEnv(EnvNodeType, "")) // default to full
	switch nodeType {
	case ServerTypeLB: // nothing
	case ServerTypePureRunner: // nothing
	default:
		// only want to activate these for full and api nodes
		defaultDB = fmt.Sprintf("sqlite3://%s/data/fn.db", curDir)
	}
	opts = append(opts, WithWebPort(getEnvInt(EnvPort, DefaultPort)))
	opts = append(opts, WithGRPCPort(getEnvInt(EnvGRPCPort, DefaultGRPCPort)))
	opts = append(opts, WithLogFormat(getEnv(EnvLogFormat, DefaultLogFormat)))
	opts = append(opts, WithLogLevel(getEnv(EnvLogLevel, DefaultLogLevel)))
	opts = append(opts, WithLogDest(getEnv(EnvLogDest, DefaultLogDest), getEnv(EnvLogPrefix, "")))
	opts = append(opts, WithZipkin(getEnv(EnvZipkinURL, "")))
	opts = append(opts, WithJaeger(getEnv(EnvJaegerURL, "")))
	opts = append(opts, WithPrometheus()) // TODO option to turn this off?
	opts = append(opts, WithDBURL(getEnv(EnvDBURL, defaultDB)))
	opts = append(opts, WithType(nodeType))

	opts = append(opts, LimitRequestBody(int64(getEnvInt(EnvMaxRequestSize, 0))))

	publicLBURL := getEnv(EnvPublicLoadBalancerURL, "")
	if publicLBURL != "" {
		logrus.Infof("using LB Base URL: '%s'", publicLBURL)
		opts = append(opts, WithTriggerAnnotator(NewStaticURLTriggerAnnotator(publicLBURL)))
		opts = append(opts, WithFnAnnotator(NewStaticURLFnAnnotator(publicLBURL)))
	} else {
		opts = append(opts, WithTriggerAnnotator(NewRequestBasedTriggerAnnotator()))
		opts = append(opts, WithFnAnnotator(NewRequestBasedFnAnnotator()))
	}

	// Agent handling depends on node type and several other options so it must be the last processed option.
	// Also we only need to create an agent if this is not an API node.
	if nodeType != ServerTypeAPI {
		opts = append(opts, WithAgentFromEnv())
	}

	return New(ctx, opts...)
}

func pwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't get working directory, possibly unsupported platform?")
	}
	// Replace forward slashes in case this is windows, URL parser errors
	return strings.Replace(cwd, "\\", "/", -1)
}

// WithWebPort maps EnvPort
func WithWebPort(port int) Option {
	return func(ctx context.Context, s *Server) error {
		if s.svcConfigs[AdminServer].Addr == s.svcConfigs[WebServer].Addr {
			s.svcConfigs[AdminServer].Addr = fmt.Sprintf(":%d", port)
		}
		s.svcConfigs[WebServer].Addr = fmt.Sprintf(":%d", port)
		return nil
	}
}

// WithGRPCPort maps EnvGRPCPort
func WithGRPCPort(port int) Option {
	return func(ctx context.Context, s *Server) error {
		s.svcConfigs[GRPCServer].Addr = fmt.Sprintf(":%d", port)
		return nil
	}
}

// WithLogFormat maps EnvLogFormat
func WithLogFormat(format string) Option {
	return func(ctx context.Context, s *Server) error {
		common.SetLogFormat(format)
		return nil
	}
}

// WithLogLevel maps EnvLogLevel
func WithLogLevel(ll string) Option {
	return func(ctx context.Context, s *Server) error {
		common.SetLogLevel(ll)
		return nil
	}
}

// WithLogDest maps EnvLogDest
func WithLogDest(dst, prefix string) Option {
	return func(ctx context.Context, s *Server) error {
		common.SetLogDest(dst, prefix)
		return nil
	}
}

// WithDBURL maps EnvDBURL
func WithDBURL(dbURL string) Option {
	return func(ctx context.Context, s *Server) error {
		if dbURL != "" {
			ds, err := datastore.New(ctx, dbURL)
			if err != nil {
				return err
			}
			s.datastore = ds
			s.lbReadAccess = agent.NewCachedDataAccess(s.datastore)
		}
		return nil
	}
}

// WithType maps EnvNodeType
func WithType(t NodeType) Option {
	return func(ctx context.Context, s *Server) error {
		s.nodeType = t
		return nil
	}
}

// WithTLS configures a service with a provided TLS configuration
func WithTLS(service string, tlsCfg *tls.Config) Option {
	return func(ctx context.Context, s *Server) error {
		s.svcConfigs[service].TLSConfig = tlsCfg
		return nil
	}
}

// WithReadDataAccess overrides the LB read DataAccess for a server
func WithReadDataAccess(ds agent.ReadDataAccess) Option {
	return func(ctx context.Context, s *Server) error {
		s.lbReadAccess = ds
		return nil
	}
}

// WithDatastore allows directly setting a datastore
func WithDatastore(ds models.Datastore) Option {
	return func(ctx context.Context, s *Server) error {
		s.datastore = ds
		if s.lbReadAccess == nil {
			s.lbReadAccess = agent.NewCachedDataAccess(ds)
		}
		return nil
	}
}

// WithAgent allows directly setting an agent
func WithAgent(agent agent.Agent) Option {
	return func(ctx context.Context, s *Server) error {
		s.agent = agent
		return nil
	}
}

func (s *Server) defaultRunnerPool() (pool.RunnerPool, error) {
	runnerAddresses := getEnv(EnvRunnerAddresses, "")
	if runnerAddresses == "" {
		return nil, errors.New("must provide FN_RUNNER_ADDRESSES  when running in default load-balanced mode")
	}
	return agent.DefaultStaticRunnerPool(strings.Split(runnerAddresses, ",")), nil
}

// WithFullAgent is a shorthand for WithAgent(... create a full agent here ...)
func WithFullAgent() Option {
	return func(ctx context.Context, s *Server) error {
		s.nodeType = ServerTypeFull

		if s.datastore == nil {
			return errors.New("full nodes must configure FN_DB_URL")
		}
		s.agent = agent.New()
		return nil
	}
}

// WithAgentFromEnv must be provided as the last server option because it relies
// on all other options being set first.
func WithAgentFromEnv() Option {
	return func(ctx context.Context, s *Server) error {
		switch s.nodeType {
		case ServerTypeAPI:
			return errors.New("should not initialize an agent for an Fn API node")
		case ServerTypePureRunner:
			if s.datastore != nil {
				return errors.New("pure runner nodes must not be configured with a datastore (FN_DB_URL)")
			}
			cancelCtx, cancel := context.WithCancel(ctx)
			prAgent, err := agent.DefaultPureRunner(cancel, s.svcConfigs[GRPCServer].Addr, s.svcConfigs[GRPCServer].TLSConfig)
			if err != nil {
				return err
			}
			s.agent = prAgent
			s.extraCtxs = append(s.extraCtxs, cancelCtx)
		case ServerTypeLB:
			s.nodeType = ServerTypeLB
			runnerURL := getEnv(EnvRunnerURL, "")
			if runnerURL == "" {
				return errors.New("no FN_RUNNER_API_URL provided for an Fn NuLB node")
			}
			if s.datastore != nil {
				return errors.New("lb nodes must not be configured with a datastore (FN_DB_URL)")
			}

			cl, err := hybrid.NewClient(runnerURL)
			if err != nil {
				return err
			}

			runnerPool, err := s.defaultRunnerPool()
			if err != nil {
				return err
			}

			// Select the placement algorithm
			placerCfg := pool.NewPlacerConfig()
			var placer pool.Placer
			switch getEnv(EnvLBPlacementAlg, "") {
			case "ch":
				placer = pool.NewCHPlacer(&placerCfg)
			default:
				placer = pool.NewNaivePlacer(&placerCfg)
			}

			s.lbReadAccess = agent.NewCachedDataAccess(cl)
			s.agent, err = agent.NewLBAgent(runnerPool, placer)
			if err != nil {
				return errors.New("LBAgent creation failed")
			}
		default:
			WithFullAgent()(ctx, s)
		}
		return nil
	}
}

// WithExtraCtx appends a context to the list of contexts the server will watch for cancellations / errors / signals.
func WithExtraCtx(extraCtx context.Context) Option {
	return func(ctx context.Context, s *Server) error {
		s.extraCtxs = append(s.extraCtxs, extraCtx)
		return nil
	}
}

//WithTriggerAnnotator adds a trigggerEndpoint provider to the server
func WithTriggerAnnotator(provider TriggerAnnotator) Option {
	return func(ctx context.Context, s *Server) error {
		s.triggerAnnotator = provider
		return nil
	}
}

//WithFnAnnotator adds a fnEndpoint provider to the server
func WithFnAnnotator(provider FnAnnotator) Option {
	return func(ctx context.Context, s *Server) error {
		s.fnAnnotator = provider
		return nil
	}
}

// WithAdminServer starts the admin server on the specified port.
func WithAdminServer(port int) Option {
	return func(ctx context.Context, s *Server) error {
		s.AdminRouter = gin.New()
		s.svcConfigs[AdminServer].Addr = fmt.Sprintf(":%d", port)
		return nil
	}
}

func WithHTTPConfig(service string, cfg *http.Server) Option {
	return func(ctx context.Context, s *Server) error {
		s.svcConfigs[service] = cfg
		return nil
	}
}

// New creates a new Functions server with the opts given. For convenience, users may
// prefer to use NewFromEnv but New is more flexible if needed.
func New(ctx context.Context, opts ...Option) *Server {
	ctx, span := trace.StartSpan(ctx, "server_init")
	defer span.End()

	log := common.Logger(ctx)
	engine := gin.New()
	s := &Server{
		Router:      engine,
		AdminRouter: engine,
		svcConfigs: map[string]*http.Server{
			WebServer: &http.Server{
				MaxHeaderBytes:    getEnvInt(EnvMaxHeaderSize, http.DefaultMaxHeaderBytes),
				ReadHeaderTimeout: getEnvDuration(EnvReadHeaderTimeout, 0),
				ReadTimeout:       getEnvDuration(EnvReadTimeout, 0),
				WriteTimeout:      getEnvDuration(EnvWriteTimeout, 0),
				IdleTimeout:       getEnvDuration(EnvHTTPIdleTimeout, 0),
			},
			AdminServer: &http.Server{},
			GRPCServer:  &http.Server{},
		},
		// Almost everything else is configured through opts (see NewFromEnv for ex.) or below
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		err := opt(ctx, s)
		if err != nil {
			log.WithError(err).Fatal("Error during server opt initialization.")
		}
	}

	if s.svcConfigs[WebServer].Addr == "" {
		s.svcConfigs[WebServer].Addr = fmt.Sprintf(":%d", DefaultPort)
	}
	if s.svcConfigs[AdminServer].Addr == "" {
		s.svcConfigs[AdminServer].Addr = fmt.Sprintf(":%d", DefaultPort)
	}
	if s.svcConfigs[GRPCServer].Addr == "" {
		s.svcConfigs[GRPCServer].Addr = fmt.Sprintf(":%d", DefaultGRPCPort)
	}

	requireConfigSet := func(id string, val interface{}) {
		if val == nil {
			log.Fatalf("Invalid configuration for server type %s, %s must be configured during startup", s.nodeType, id)
		}
	}
	requireConfigNotSet := func(id string, val interface{}) {
		if val != nil {
			log.Fatalf("Invalid configuration for server type %s, %s must not be configured during startup", s.nodeType, id)
		}
	}

	// Check that WithAgent options have been processed correctly.
	// Yuck the yuck  - server should really be split into several interfaces (LB, Runner, API) and each should be instantiated separately
	switch s.nodeType {
	case ServerTypeAPI:
		requireConfigNotSet("agent", s.agent)
		requireConfigSet("datastore", s.datastore)
		requireConfigSet("triggerAnnotator", s.triggerAnnotator)
	case ServerTypeFull:
		requireConfigSet("agent", s.agent)
		requireConfigSet("lbReadAccess", s.lbReadAccess)
		requireConfigSet("datastore", s.datastore)
		requireConfigSet("triggerAnnotator", s.triggerAnnotator)

	case ServerTypeLB:
		requireConfigSet("lbReadAccess", s.lbReadAccess)
		requireConfigSet("agent", s.agent)

	case ServerTypePureRunner:
		requireConfigSet("agent", s.agent)

	default:

		log.Fatal("unknown server type %d", s.nodeType)

	}

	s.Router.Use(loggerWrap, traceWrap) // TODO should be opts
	optionalCorsWrap(s.Router)          // TODO should be an opt
	apiMetricsWrap(s)
	// panicWrap is last, specifically so that logging, tracing, cors, metrics, etc wrappers run
	s.Router.Use(panicWrap)
	s.AdminRouter.Use(panicWrap)
	s.bindHandlers(ctx)

	s.appListeners = new(appListeners)
	s.fnListeners = new(fnListeners)
	s.triggerListeners = new(triggerListeners)

	// TODO it's not clear that this is always correct as the read store  won't  get wrapping
	s.datastore = datastore.Wrap(s.datastore)
	s.datastore = fnext.NewDatastore(s.datastore, s.appListeners, s.fnListeners, s.triggerListeners)

	return s
}

// WithPrometheus activates the prometheus collection and /metrics endpoint
func WithPrometheus() Option {
	return func(ctx context.Context, s *Server) error {
		reg := promclient.NewRegistry()
		reg.MustRegister(promclient.NewProcessCollector(promclient.ProcessCollectorOpts{
			PidFn:     func() (int, error) { return os.Getpid(), nil },
			Namespace: "fn",
		}),
			promclient.NewGoCollector(),
		)

		for _, exeName := range getMonitoredCmdNames() {
			san := promSanitizeMetricName(exeName)

			err := reg.Register(promclient.NewProcessCollector(promclient.ProcessCollectorOpts{
				PidFn:     getPidCmd(exeName),
				Namespace: san,
			}))
			if err != nil {
				panic(err)
			}
		}

		exporter, err := prometheus.NewExporter(prometheus.Options{
			Namespace: "fn",
			Registry:  reg,
			OnError:   func(err error) { logrus.WithError(err).Error("opencensus prometheus exporter err") },
		})
		if err != nil {
			return fmt.Errorf("error starting prometheus exporter: %v", err)
		}
		s.promExporter = exporter
		view.RegisterExporter(exporter)

		return nil
	}
}

// WithoutHTTPTriggerEndpoints optionally disables the trigger and route endpoints from a LB -supporting server, allowing extensions to replace them with their own versions
func WithoutHTTPTriggerEndpoints() Option {
	return func(ctx context.Context, s *Server) error {
		s.noHTTTPTriggerEndpoint = true
		return nil
	}
}

// WithoutFnInvokeEndpoints optionally disables the fn direct invoke endpoints from a LB -supporting server, allowing extensions to replace them with their own versions
func WithoutFnInvokeEndpoints() Option {
	return func(ctx context.Context, s *Server) error {
		s.noFnInvokeEndpoint = true
		return nil
	}
}

// WithJaeger maps EnvJaegerURL
func WithJaeger(jaegerURL string) Option {
	return func(ctx context.Context, s *Server) error {
		// ex: "http://localhost:14268"
		if jaegerURL == "" {
			return nil
		}

		exporter, err := jaeger.NewExporter(jaeger.Options{
			Endpoint:    jaegerURL,
			ServiceName: "fn",
		})
		if err != nil {
			return fmt.Errorf("error connecting to jaeger: %v", err)
		}
		trace.RegisterExporter(exporter)
		logrus.WithFields(logrus.Fields{"url": jaegerURL}).Info("exporting spans to jaeger")

		// TODO don't do this. testing parity.
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
		return nil
	}
}

// WithZipkin maps EnvZipkinURL
func WithZipkin(zipkinURL string) Option {
	return func(ctx context.Context, s *Server) error {
		// ex: "http://zipkin:9411/api/v2/spans"

		if zipkinURL == "" {
			return nil
		}

		reporter := zipkinhttp.NewReporter(zipkinURL, zipkinhttp.MaxBacklog(10000))
		exporter := zipkin.NewExporter(reporter, nil)
		trace.RegisterExporter(exporter)
		logrus.WithFields(logrus.Fields{"url": zipkinURL}).Info("exporting spans to zipkin")

		// TODO don't do this. testing parity.
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
		return nil
	}
}

// prometheus only allows [a-zA-Z0-9:_] in metrics names.
func promSanitizeMetricName(name string) string {
	res := make([]rune, 0, len(name))
	for _, rVal := range name {
		if unicode.IsDigit(rVal) || unicode.IsLetter(rVal) || rVal == ':' {
			res = append(res, rVal)
		} else {
			res = append(res, '_')
		}
	}
	return string(res)
}

// determine sidecar-monitored cmd names. But by default
// we track dockerd + containerd
func getMonitoredCmdNames() []string {

	// override? empty variable to disable trackers
	val, ok := os.LookupEnv(EnvProcessCollectorList)
	if ok {
		return strings.Fields(val)
	}

	// by default, we monitor dockerd and containerd
	return []string{"dockerd", "docker-containerd"}
}

// TODO plumbing considerations, we've put the S pipe next to the chandalier...
func getPidCmd(cmdName string) func() (int, error) {
	// prometheus' process collector only works on linux anyway. let them do the
	// process detection, if we return an error here we just get 0 metrics and it
	// does not log / blow up (that's fine!) it's also likely we hit permissions
	// errors here for many installations, we want to do similar and ignore (we
	// just want for prod).

	var pid int

	return func() (int, error) {
		if pid != 0 {
			// make sure it's our pid.
			if isPidMatchCmd(cmdName, pid) {
				return pid, nil
			}
			pid = 0 // reset to go search
		}

		if pids, err := getPidList(); err == nil {
			for _, test := range pids {
				if isPidMatchCmd(cmdName, test) {
					pid = test
					return pid, nil
				}
			}
		}

		return pid, io.EOF
	}
}

func isPidMatchCmd(cmdName string, pid int) bool {
	fs, err := os.Open("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil {
		return false
	}
	defer fs.Close()

	rd := bufio.NewReader(fs)
	tok, err := rd.ReadSlice(0)
	if err != nil || len(tok) < len(cmdName) {
		return false
	}

	return filepath.Base(string(tok[:len(tok)-1])) == cmdName
}

func getPidList() ([]int, error) {
	var pids []int
	dir, err := os.Open("/proc")
	if err != nil {
		return pids, nil
	}
	defer dir.Close()

	files, err := dir.Readdirnames(0)
	if err != nil {
		return pids, nil
	}

	pids = make([]int, 0, len(files))
	for _, tok := range files {
		if conv, err := strconv.ParseUint(tok, 10, 64); err == nil {
			pids = append(pids, int(conv))
		}
	}
	return pids, nil
}

func extractFields(c *gin.Context) logrus.Fields {
	fields := logrus.Fields{"action": path.Base(c.HandlerName())}
	for _, param := range c.Params {
		fields[param.Key] = param.Value
	}
	return fields
}

// Start runs any configured machinery, including the http server, agent, etc.
// Start will block until the context is cancelled or times out.
func (s *Server) Start(ctx context.Context) {
	newctx, cancel := contextWithSignal(ctx, os.Interrupt, syscall.SIGTERM)
	s.startGears(newctx, cancel)
}

func (s *Server) startGears(ctx context.Context, cancel context.CancelFunc) {

	const runHeader = `
        ______
       / ____/___
      / /_  / __ \
     / __/ / / / /
    /_/   /_/ /_/`
	fmt.Println(runHeader)
	fmt.Printf("        v%s\n\n", version.Version)

	logrus.WithField("type", s.nodeType).Infof("Fn serving on `%v`", s.svcConfigs[WebServer].Addr)

	installChildReaper()

	server := s.svcConfigs[WebServer]
	if server.Handler == nil {
		server.Handler = &ochttp.Handler{Handler: s.Router}
	}

	go func() {
		var err error
		if server.TLSConfig != nil {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("server error")
			cancel()
		} else {
			logrus.Info("server stopped")
		}
	}()

	if s.svcConfigs[WebServer].Addr != s.svcConfigs[AdminServer].Addr {
		logrus.WithField("type", s.nodeType).Infof("Fn Admin serving on `%v`", s.svcConfigs[AdminServer].Addr)
		adminServer := s.svcConfigs[AdminServer]
		if adminServer.Handler == nil {
			adminServer.Handler = &ochttp.Handler{Handler: s.AdminRouter}
		}

		go func() {
			var err error
			if adminServer.TLSConfig != nil {
				err = adminServer.ListenAndServeTLS("", "")
			} else {
				err = adminServer.ListenAndServe()
			}
			if err != nil && err != http.ErrServerClosed {
				logrus.WithError(err).Error("server error")
				cancel()
			} else {
				logrus.Info("server stopped")
			}
		}()

		defer func() {
			if err := adminServer.Shutdown(context.Background()); err != nil {
				logrus.WithError(err).Error("admin server shutdown error")
			}
		}()
	}

	// listening for signals or listener errors or cancellations on all registered contexts.
	s.extraCtxs = append(s.extraCtxs, ctx)
	cases := make([]reflect.SelectCase, len(s.extraCtxs))
	for i, ctx := range s.extraCtxs {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())}
	}
	nth, recv, wasSend := reflect.Select(cases)
	if wasSend {
		logrus.WithFields(logrus.Fields{
			"ctx_number":     nth,
			"received_value": recv.String(),
		}).Debug("Stopping because of received value from done context.")
	} else {
		logrus.WithFields(logrus.Fields{
			"ctx_number": nth,
		}).Debug("Stopping because of closed channel from done context.")
	}

	// TODO: do not wait forever during graceful shutdown (add graceful shutdown timeout)
	if err := server.Shutdown(context.Background()); err != nil {
		logrus.WithError(err).Error("server shutdown error")
	}

	if s.agent != nil {
		err := s.agent.Close() // after we stop taking requests, wait for all tasks to finish
		if err != nil {
			logrus.WithError(err).Error("Fail to close the agent")
		}
	}
}

func (s *Server) goneResponse(c *gin.Context) {
	c.Status(http.StatusGone)
}

func (s *Server) bindHandlers(ctx context.Context) {
	engine := s.Router
	admin := s.AdminRouter
	// now for extensible middleware
	engine.Use(s.rootMiddlewareWrapper())

	engine.GET("/", handlePing)
	admin.GET("/version", handleVersion)

	if s.promExporter != nil {
		admin.GET("/metrics", gin.WrapH(s.promExporter))
	}

	profilerSetup(admin, "/debug")

	// Pure runners don't have any route, they have grpc
	switch s.nodeType {

	case ServerTypeFull, ServerTypeAPI:
		cleanv2 := engine.Group("/v2")
		v2 := cleanv2.Group("")
		v2.Use(s.apiMiddlewareWrapper())

		{
			v2.GET("/apps", s.handleAppList)
			v2.POST("/apps", s.handleAppCreate)
			v2.GET("/apps/:app_id", s.handleAppGet)
			v2.PUT("/apps/:app_id", s.handleAppUpdate)
			v2.DELETE("/apps/:app_id", s.handleAppDelete)

			v2.GET("/fns", s.handleFnList)
			v2.POST("/fns", s.handleFnCreate)
			v2.GET("/fns/:fn_id", s.handleFnGet)
			v2.PUT("/fns/:fn_id", s.handleFnUpdate)
			v2.DELETE("/fns/:fn_id", s.handleFnDelete)

			v2.GET("/triggers", s.handleTriggerList)
			v2.POST("/triggers", s.handleTriggerCreate)
			v2.GET("/triggers/:trigger_id", s.handleTriggerGet)
			v2.PUT("/triggers/:trigger_id", s.handleTriggerUpdate)
			v2.DELETE("/triggers/:trigger_id", s.handleTriggerDelete)
		}

		// TODO remove these in 30 days or something
		v2.GET("/fns/:fn_id/calls", s.goneResponse)
		v2.GET("/fns/:fn_id/calls/:call_id", s.goneResponse)
		v2.GET("/fns/:fn_id/calls/:call_id/log", s.goneResponse)

		runner := cleanv2.Group("/runner")

		runnerAppAPI := runner.Group("/apps/:app_id")
		runnerAppAPI.Use(setAppIDInCtx)
		// TODO figure out how to deprecate
		runnerAppAPI.GET("/triggerBySource/:trigger_type/*trigger_source", s.handleRunnerGetTriggerBySource)
	}

	switch s.nodeType {
	case ServerTypeFull, ServerTypeLB:
		if !s.noHTTTPTriggerEndpoint {
			lbTriggerGroup := engine.Group("/t")
			lbTriggerGroup.Any("/:app_name", s.handleHTTPTriggerCall)
			lbTriggerGroup.Any("/:app_name/*trigger_source", s.handleHTTPTriggerCall)
		}

		if !s.noFnInvokeEndpoint {
			lbFnInvokeGroup := engine.Group("/invoke")
			lbFnInvokeGroup.POST("/:fn_id", s.handleFnInvokeCall)
		}
	}

	engine.NoRoute(func(c *gin.Context) {
		var e models.APIError = models.ErrPathNotFound
		err := models.NewAPIError(e.Code(), fmt.Errorf("%v: %s %s", e.Error(), c.Request.Method, c.Request.URL.Path))
		handleErrorResponse(c, err)
	})

	engine.HandleMethodNotAllowed = true
	engine.NoMethod(func(c *gin.Context) {
		var e models.APIError = models.ErrMethodNotAllowed
		err := models.NewAPIError(e.Code(), fmt.Errorf("%v: %s %s", e.Error(), c.Request.Method, c.Request.URL.Path))
		handleErrorResponse(c, err)
	})

}

// Datastore implements fnext.ExtServer
func (s *Server) Datastore() models.Datastore {
	return s.datastore
}

// Agent implements fnext.ExtServer
func (s *Server) Agent() agent.Agent {
	return s.agent
}

func pageParams(c *gin.Context) (cursor string, perPage int) {
	cursor = c.Query("cursor")

	perPage, _ = strconv.Atoi(c.Query("per_page"))
	if perPage > 100 {
		perPage = 100
	} else if perPage <= 0 {
		perPage = 30
	}
	return cursor, perPage
}
