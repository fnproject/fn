package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/hybrid"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/fnproject/fn/api/version"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/exporter/prometheus"
	"go.opencensus.io/exporter/zipkin"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

const (
	EnvLogLevel  = "FN_LOG_LEVEL"
	EnvLogDest   = "FN_LOG_DEST"
	EnvLogPrefix = "FN_LOG_PREFIX"
	EnvMQURL     = "FN_MQ_URL"
	EnvDBURL     = "FN_DB_URL"
	EnvLOGDBURL  = "FN_LOGSTORE_URL"
	EnvRunnerURL = "FN_RUNNER_API_URL"
	EnvNodeType  = "FN_NODE_TYPE"
	EnvPort      = "FN_PORT" // be careful, Gin expects this variable to be "port"
	EnvAPICORS   = "FN_API_CORS"
	EnvZipkinURL = "FN_ZIPKIN_URL"

	// Defaults
	DefaultLogLevel = "info"
	DefaultLogDest  = "stderr"
	DefaultPort     = 8080
)

type ServerNodeType int32

const (
	ServerTypeFull ServerNodeType = iota
	ServerTypeAPI
	ServerTypeRunner
)

func (s ServerNodeType) String() string {
	switch s {
	default:
		return "full"
	case ServerTypeAPI:
		return "api"
	case ServerTypeRunner:
		return "runner"
	}
}

type Server struct {
	// TODO this one maybe we have `AddRoute` in extensions?
	Router *gin.Engine

	agent           agent.Agent
	datastore       models.Datastore
	mq              models.MessageQueue
	logstore        models.LogStore
	nodeType        ServerNodeType
	appListeners    *appListeners
	rootMiddlewares []fnext.Middleware
	apiMiddlewares  []fnext.Middleware
}

func nodeTypeFromString(value string) ServerNodeType {
	switch value {
	case "api":
		return ServerTypeAPI
	case "runner":
		return ServerTypeRunner
	default:
		return ServerTypeFull
	}
}

// NewFromEnv creates a new Functions server based on env vars.
func NewFromEnv(ctx context.Context, opts ...ServerOption) *Server {
	curDir := pwd()
	var defaultDB, defaultMQ string
	nodeType := nodeTypeFromString(getEnv(EnvNodeType, "")) // default to full
	if nodeType != ServerTypeRunner {
		// only want to activate these for full and api nodes
		defaultDB = fmt.Sprintf("sqlite3://%s/data/fn.db", curDir)
		defaultMQ = fmt.Sprintf("bolt://%s/data/fn.mq", curDir)
	}
	opts = append(opts, WithLogLevel(getEnv(EnvLogLevel, DefaultLogLevel)))
	opts = append(opts, WithLogDest(getEnv(EnvLogDest, DefaultLogDest), getEnv(EnvLogPrefix, "")))
	opts = append(opts, WithTracer(getEnv(EnvZipkinURL, ""))) // do this early on, so below can use these
	opts = append(opts, WithDBURL(getEnv(EnvDBURL, defaultDB)))
	opts = append(opts, WithMQURL(getEnv(EnvMQURL, defaultMQ)))
	opts = append(opts, WithLogURL(getEnv(EnvLOGDBURL, "")))
	opts = append(opts, WithRunnerURL(getEnv(EnvRunnerURL, "")))
	opts = append(opts, WithType(nodeType))
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

func WithLogLevel(ll string) ServerOption {
	return func(ctx context.Context, s *Server) error {
		common.SetLogLevel(ll)
		return nil
	}
}

func WithLogDest(dst, prefix string) ServerOption {
	return func(ctx context.Context, s *Server) error {
		common.SetLogDest(dst, prefix)
		return nil
	}
}

func WithDBURL(dbURL string) ServerOption {
	return func(ctx context.Context, s *Server) error {
		if dbURL != "" {
			ds, err := datastore.New(ctx, dbURL)
			if err != nil {
				return err
			}
			s.datastore = ds
		}
		return nil
	}
}

func WithMQURL(mqURL string) ServerOption {
	return func(ctx context.Context, s *Server) error {
		if mqURL != "" {
			mq, err := mqs.New(mqURL)
			if err != nil {
				return err
			}
			s.mq = mq
		}
		return nil
	}
}

func WithLogURL(logstoreURL string) ServerOption {
	return func(ctx context.Context, s *Server) error {
		if ldb := logstoreURL; ldb != "" {
			logDB, err := logs.New(ctx, logstoreURL)
			if err != nil {
				return err
			}
			s.logstore = logDB
		}
		return nil
	}
}

func WithRunnerURL(runnerURL string) ServerOption {
	return func(ctx context.Context, s *Server) error {
		if runnerURL != "" {
			cl, err := hybrid.NewClient(runnerURL)
			if err != nil {
				return err
			}
			s.agent = agent.New(agent.NewCachedDataAccess(cl))
		}
		return nil
	}
}

func WithType(t ServerNodeType) ServerOption {
	return func(ctx context.Context, s *Server) error {
		s.nodeType = t
		return nil
	}
}

func WithDatastore(ds models.Datastore) ServerOption {
	return func(ctx context.Context, s *Server) error {
		s.datastore = ds
		return nil
	}
}

func WithMQ(mq models.MessageQueue) ServerOption {
	return func(ctx context.Context, s *Server) error {
		s.mq = mq
		return nil
	}
}

func WithLogstore(ls models.LogStore) ServerOption {
	return func(ctx context.Context, s *Server) error {
		s.logstore = ls
		return nil
	}
}

func WithAgent(agent agent.Agent) ServerOption {
	return func(ctx context.Context, s *Server) error {
		s.agent = agent
		return nil
	}
}

// New creates a new Functions server with the opts given. For convenience, users may
// prefer to use NewFromEnv but New is more flexible if needed.
func New(ctx context.Context, opts ...ServerOption) *Server {
	ctx, span := trace.StartSpan(ctx, "server_init")
	defer span.End()

	log := common.Logger(ctx)
	s := &Server{
		Router: gin.New(),
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

	if s.logstore == nil { // TODO seems weird?
		s.logstore = s.datastore
	}

	// TODO we maybe should use the agent.DirectDataAccess in the /runner endpoints server side?

	switch s.nodeType {
	case ServerTypeAPI:
		if s.agent != nil {
			log.Info("shutting down agent configured for api node")
			s.agent.Close()
		}
		s.agent = nil
	case ServerTypeRunner:
		if s.agent == nil {
			log.Fatal("No agent started for a runner node, add FN_RUNNER_API_URL to configuration.")
		}
	default:
		s.nodeType = ServerTypeFull
		if s.datastore == nil || s.logstore == nil || s.mq == nil {
			log.Fatal("Full nodes must configure FN_DB_URL, FN_LOG_URL, FN_MQ_URL.")
		}

		// TODO force caller to use WithAgent option? at this point we need to use the ds/ls/mq configured after all opts are run.
		if s.agent == nil {
			// TODO for tests we don't want cache, really, if we force WithAgent this can be fixed. cache needs to be moved anyway so that runner nodes can use it...
			s.agent = agent.New(agent.NewCachedDataAccess(agent.NewDirectDataAccess(s.datastore, s.logstore, s.mq)))
		}
	}

	setMachineID()
	s.Router.Use(loggerWrap, traceWrap, panicWrap) // TODO should be opts
	optionalCorsWrap(s.Router)                     // TODO should be an opt
	s.bindHandlers(ctx)

	s.appListeners = new(appListeners)
	s.datastore = fnext.NewDatastore(s.datastore, s.appListeners)

	return s
}

// TODO need to fix this to handle the nil case better
func WithTracer(zipkinURL string) ServerOption {
	return func(ctx context.Context, s *Server) error {
		var (
			// TODO add server identifier to this crap
			//debugMode          = false
			//serviceName        = "fnserver"
			//serviceHostPort    = "localhost:8080" // meh
			zipkinHTTPEndpoint = zipkinURL
			// ex: "http://zipkin:9411/api/v2/spans"
		)

		if zipkinHTTPEndpoint != "" {
			reporter := zipkinhttp.NewReporter(zipkinURL, zipkinhttp.MaxBacklog(10000))
			exporter := zipkin.NewExporter(reporter, nil)
			trace.RegisterExporter(exporter)
			logrus.WithFields(logrus.Fields{"url": zipkinHTTPEndpoint}).Info("exporting spans to zipkin")

			// TODO don't do this. testing parity.
			trace.SetDefaultSampler(trace.AlwaysSample())
		}

		return nil
	}
}

func setMachineID() {
	port := uint16(getEnvInt(EnvPort, DefaultPort))
	addr := whoAmI().To4()
	if addr == nil {
		addr = net.ParseIP("127.0.0.1").To4()
		logrus.Warn("could not find non-local ipv4 address to use, using '127.0.0.1' for ids, if this is a cluster beware of duplicate ids!")
	}
	id.SetMachineIdHost(addr, port)
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

func extractFields(c *gin.Context) logrus.Fields {
	fields := logrus.Fields{"action": path.Base(c.HandlerName())}
	for _, param := range c.Params {
		fields[param.Key] = param.Value
	}
	return fields
}

func (s *Server) Start(ctx context.Context) {
	newctx, cancel := contextWithSignal(ctx, os.Interrupt, syscall.SIGTERM)
	s.startGears(newctx, cancel)
}

func (s *Server) startGears(ctx context.Context, cancel context.CancelFunc) {
	// By default it serves on :8080 unless a
	// FN_PORT environment variable was defined.
	listen := fmt.Sprintf(":%d", getEnvInt(EnvPort, DefaultPort))

	const runHeader = `
        ______
       / ____/___
      / /_  / __ \
     / __/ / / / /
    /_/   /_/ /_/`
	fmt.Println(runHeader)
	fmt.Printf("        v%s\n\n", version.Version)

	logrus.WithField("type", s.nodeType).Infof("Fn serving on `%v`", listen)

	installChildReaper()

	server := http.Server{
		Addr:    listen,
		Handler: &ochttp.Handler{Handler: s.Router},

		// TODO we should set read/write timeouts
	}

	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("server error")
			cancel()
		} else {
			logrus.Info("server stopped")
		}
	}()

	// listening for signals or listener errors...
	<-ctx.Done()

	// TODO: do not wait forever during graceful shutdown (add graceful shutdown timeout)
	if err := server.Shutdown(context.Background()); err != nil {
		logrus.WithError(err).Error("server shutdown error")
	}

	if s.agent != nil {
		s.agent.Close() // after we stop taking requests, wait for all tasks to finish
	}
}

func (s *Server) bindHandlers(ctx context.Context) {
	engine := s.Router
	// now for extendible middleware
	engine.Use(s.rootMiddlewareWrapper())

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)

	exporter, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "fn",
		OnError:   func(err error) { logrus.WithError(err).Error("opencensus prometheus exporter err") },
	})
	if err != nil {
		log.Fatal(err)
	}
	view.RegisterExporter(exporter)

	// TODO: move under v1 ?
	engine.GET("/metrics", gin.WrapH(exporter))

	profilerSetup(engine, "/debug")

	if s.nodeType != ServerTypeRunner {
		v1 := engine.Group("/v1")
		v1.Use(setAppNameInCtx)
		v1.Use(s.apiMiddlewareWrapper())
		v1.GET("/apps", s.handleAppList)
		v1.POST("/apps", s.handleAppCreate)

		{
			apps := v1.Group("/apps/:app")
			apps.Use(appNameCheck)

			apps.GET("", s.handleAppGet)
			apps.PATCH("", s.handleAppUpdate)
			apps.DELETE("", s.handleAppDelete)

			apps.GET("/routes", s.handleRouteList)
			apps.POST("/routes", s.handleRoutesPostPutPatch)
			apps.GET("/routes/:route", s.handleRouteGet)
			apps.PATCH("/routes/*route", s.handleRoutesPostPutPatch)
			apps.PUT("/routes/*route", s.handleRoutesPostPutPatch)
			apps.DELETE("/routes/*route", s.handleRouteDelete)

			apps.GET("/calls", s.handleCallList)

			apps.GET("/calls/:call", s.handleCallGet)
			apps.GET("/calls/:call/log", s.handleCallLogGet)
		}

		{
			runner := v1.Group("/runner")
			runner.PUT("/async", s.handleRunnerEnqueue)
			runner.GET("/async", s.handleRunnerDequeue)

			runner.POST("/start", s.handleRunnerStart)
			runner.POST("/finish", s.handleRunnerFinish)
		}
	}

	if s.nodeType != ServerTypeAPI {
		runner := engine.Group("/r")
		runner.Use(appNameCheck)
		runner.Any("/:app", s.handleFunctionCall)
		runner.Any("/:app/*route", s.handleFunctionCall)
	}

	engine.NoRoute(func(c *gin.Context) {
		var err error
		switch {
		case s.nodeType == ServerTypeAPI && strings.HasPrefix(c.Request.URL.Path, "/r/"):
			err = models.ErrInvokeNotSupported
		case s.nodeType == ServerTypeRunner && strings.HasPrefix(c.Request.URL.Path, "/v1/"):
			err = models.ErrAPINotSupported
		default:
			var e models.APIError = models.ErrPathNotFound
			err = models.NewAPIError(e.Code(), fmt.Errorf("%v: %s", e.Error(), c.Request.URL.Path))
		}
		handleErrorResponse(c, err)
	})
}

// implements fnext.ExtServer
func (s *Server) Datastore() models.Datastore {
	return s.datastore
}

// returns the unescaped ?cursor and ?perPage values
// pageParams clamps 0 < ?perPage <= 100 and defaults to 30 if 0
// ignores parsing errors and falls back to defaults.
func pageParams(c *gin.Context, base64d bool) (cursor string, perPage int) {
	cursor = c.Query("cursor")
	if base64d {
		cbytes, _ := base64.RawURLEncoding.DecodeString(cursor)
		cursor = string(cbytes)
	}

	perPage, _ = strconv.Atoi(c.Query("per_page"))
	if perPage > 100 {
		perPage = 100
	} else if perPage <= 0 {
		perPage = 30
	}
	return cursor, perPage
}

type appResponse struct {
	Message string      `json:"message"`
	App     *models.App `json:"app"`
}

type appsResponse struct {
	Message    string        `json:"message"`
	NextCursor string        `json:"next_cursor"`
	Apps       []*models.App `json:"apps"`
}

type routeResponse struct {
	Message string        `json:"message"`
	Route   *models.Route `json:"route"`
}

type routesResponse struct {
	Message    string          `json:"message"`
	NextCursor string          `json:"next_cursor"`
	Routes     []*models.Route `json:"routes"`
}

type callResponse struct {
	Message string       `json:"message"`
	Call    *models.Call `json:"call"`
}

type callsResponse struct {
	Message    string         `json:"message"`
	NextCursor string         `json:"next_cursor"`
	Calls      []*models.Call `json:"calls"`
}
