package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"syscall"

	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/agent/hybrid"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/datastore/cache"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/fnproject/fn/api/version"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/sirupsen/logrus"
)

var (
	currDir string
)

const (
	EnvLogLevel  = "FN_LOG_LEVEL"
	EnvMQURL     = "FN_MQ_URL"
	EnvDBURL     = "FN_DB_URL"
	EnvLOGDBURL  = "FN_LOGSTORE_URL"
	EnvRunnerURL = "FN_RUNNER_URL"
	EnvNodeType  = "FN_NODE_TYPE"
	EnvPort      = "FN_PORT" // be careful, Gin expects this variable to be "port"
	EnvAPICORS   = "FN_API_CORS"
	EnvZipkinURL = "FN_ZIPKIN_URL"

	// Defaults
	DefaultLogLevel = "info"
	DefaultPort     = 8080
)

type ServerNodeType int32

const (
	ServerTypeFull ServerNodeType = iota
	ServerTypeAPI
	ServerTypeRunner
)

type Server struct {
	Router          *gin.Engine
	Agent           agent.Agent
	Datastore       models.Datastore
	MQ              models.MessageQueue
	LogDB           models.LogStore
	nodeType        ServerNodeType
	appListeners    []fnext.AppListener
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
	opts = append(opts, WithDBURL(getEnv(EnvDBURL, fmt.Sprintf("sqlite3://%s/data/fn.db", currDir))))
	opts = append(opts, WithMQURL(getEnv(EnvMQURL, fmt.Sprintf("bolt://%s/data/fn.mq", currDir))))
	opts = append(opts, WithLogURL(getEnv(EnvLOGDBURL, "")))
	opts = append(opts, WithRunnerURL(getEnv(EnvRunnerURL, "")))
	opts = append(opts, WithType(nodeTypeFromString(getEnv(EnvNodeType, ""))))
	return New(ctx, opts...)
}

func WithDBURL(dbURL string) ServerOption {
	if dbURL != "" {
		ds, err := datastore.New(dbURL)
		if err != nil {
			logrus.WithError(err).Fatalln("Error initializing datastore.")
		}
		return WithDatastore(ds)
	}
	return noop
}

func WithMQURL(mqURL string) ServerOption {
	if mqURL != "" {
		mq, err := mqs.New(mqURL)
		if err != nil {
			logrus.WithError(err).Fatal("Error initializing message queue.")
		}
		return WithMQ(mq)
	}
	return noop
}

func WithLogURL(logstoreURL string) ServerOption {
	if ldb := logstoreURL; ldb != "" {
		logDB, err := logs.New(logstoreURL)
		if err != nil {
			logrus.WithError(err).Fatal("Error initializing logs store.")
		}
		return WithLogstore(logDB)
	}
	return noop
}

func WithRunnerURL(runnerURL string) ServerOption {
	if runnerURL != "" {
		cl, err := hybrid.NewClient(runnerURL)
		if err != nil {
			logrus.WithError(err).Fatal("Error initializing runner API client.")
		}
		return WithAgent(agent.New(cl))
	}
	return noop
}

func noop(s *Server) {}

func WithType(t ServerNodeType) ServerOption {
	return func(s *Server) { s.nodeType = t }
}

func WithDatastore(ds models.Datastore) ServerOption {
	return func(s *Server) { s.Datastore = ds }
}

func WithMQ(mq models.MessageQueue) ServerOption {
	return func(s *Server) { s.MQ = mq }
}

func WithLogstore(ls models.LogStore) ServerOption {
	return func(s *Server) { s.LogDB = ls }
}

func WithAgent(agent agent.Agent) ServerOption {
	return func(s *Server) { s.Agent = agent }
}

// New creates a new Functions server with the opts given. Use NewFromENV or NewFromURLs for more
// convenience.
func New(ctx context.Context, opts ...ServerOption) *Server {
	setTracer() // NOTE: this is first, if agent was started before this is done, we got paniced. TODO should be an opt

	s := &Server{
		Router: gin.New(),
		// Almost everything else is configured through opts (see NewFromEnv for ex.) or below
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(s)
	}

	if s.LogDB == nil { // TODO seems weird?
		s.LogDB = s.Datastore
	}

	// TODO we maybe should use the agent.DirectDataAccess in the /runner endpoints server side?

	switch s.nodeType {
	case ServerTypeAPI:
		s.Agent = nil
	case ServerTypeRunner:
		if s.Agent == nil {
			logrus.Fatal("No agent started for a runner node, add FN_RUNNER_URL to configuration.")
		}
	default:
		s.nodeType = ServerTypeFull
		if s.Datastore == nil || s.LogDB == nil || s.MQ == nil {
			logrus.Fatal("Full nodes must configure FN_DB_URL, FN_LOG_URL, FN_MQ_URL.")
		}

		// TODO force caller to use WithAgent option ?
		// TODO for tests we don't want cache, really, if we force WithAgent this can be fixed. cache needs to be moved anyway so that runner nodes can use it...
		s.Agent = agent.New(agent.NewDirectDataAccess(cache.Wrap(s.Datastore), s.LogDB, s.MQ))
	}

	// NOTE: testServer() in tests doesn't use these, need to change tests
	setMachineID()
	s.Router.Use(loggerWrap, traceWrap, panicWrap) // TODO should be opts
	optionalCorsWrap(s.Router)                     // TODO should be an opt
	s.bindHandlers(ctx)
	return s
}

func setTracer() {
	var (
		debugMode          = false
		serviceName        = "fnserver"
		serviceHostPort    = "localhost:8080" // meh
		zipkinHTTPEndpoint = getEnv(EnvZipkinURL, "")
		// ex: "http://zipkin:9411/api/v1/spans"
	)

	var collector zipkintracer.Collector

	// custom Zipkin collector to send tracing spans to Prometheus
	promCollector, promErr := NewPrometheusCollector()
	if promErr != nil {
		logrus.WithError(promErr).Fatalln("couldn't start Prometheus trace collector")
	}

	logger := zipkintracer.LoggerFunc(func(i ...interface{}) error { logrus.Error(i...); return nil })

	if zipkinHTTPEndpoint != "" {
		// Custom PrometheusCollector and Zipkin HTTPCollector
		httpCollector, zipErr := zipkintracer.NewHTTPCollector(zipkinHTTPEndpoint, zipkintracer.HTTPLogger(logger))
		if zipErr != nil {
			logrus.WithError(zipErr).Fatalln("couldn't start Zipkin trace collector")
		}
		collector = zipkintracer.MultiCollector{httpCollector, promCollector}
	} else {
		// Custom PrometheusCollector only
		collector = promCollector
	}

	ziptracer, err := zipkintracer.NewTracer(zipkintracer.NewRecorder(collector, debugMode, serviceHostPort, serviceName),
		zipkintracer.ClientServerSameSpan(true),
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't start tracer")
	}

	// wrap the Zipkin tracer in a FnTracer which will also send spans to Prometheus
	fntracer := NewFnTracer(ziptracer)

	opentracing.SetGlobalTracer(fntracer)
	logrus.WithFields(logrus.Fields{"url": zipkinHTTPEndpoint}).Info("started tracer")
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

	logrus.Infof("Serving Functions API on address `%s`", listen)

	server := http.Server{
		Addr:    listen,
		Handler: s.Router,
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

	s.Agent.Close() // after we stop taking requests, wait for all tasks to finish
}

func (s *Server) bindHandlers(ctx context.Context) {
	engine := s.Router
	// now for extendible middleware
	engine.Use(s.rootMiddlewareWrapper())

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)
	// TODO: move the following under v1
	engine.GET("/stats", s.handleStats)
	engine.GET("/metrics", s.handlePrometheusMetrics)

	if s.nodeType != ServerTypeRunner {
		v1 := engine.Group("/v1")
		v1.Use(s.apiMiddlewareWrapper())
		v1.GET("/apps", s.handleAppList)
		v1.POST("/apps", s.handleAppCreate)

		{
			apps := v1.Group("/apps/:app")
			apps.Use(appWrap)

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
		runner.Use(appWrap)
		runner.Any("/:app", s.handleFunctionCall)
		runner.Any("/:app/*route", s.handleFunctionCall)
	}

	engine.NoRoute(func(c *gin.Context) {
		logrus.Debugln("not found", c.Request.URL.Path)
		c.JSON(http.StatusNotFound, simpleError(errors.New("Path not found")))
	})
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

type callLogResponse struct {
	Message string          `json:"message"`
	Log     *models.CallLog `json:"log"`
}
