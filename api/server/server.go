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

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/agent"
	"github.com/fnproject/fn/api/common"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/datastore/cache"
	"github.com/fnproject/fn/api/extenders"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/fnproject/fn/api/version"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	EnvLogLevel  = "log_level"
	EnvMQURL     = "mq_url"
	EnvDBURL     = "db_url"
	EnvLOGDBURL  = "logstore_url"
	EnvPort      = "port" // be careful, Gin expects this variable to be "port"
	EnvAPIURL    = "api_url"
	EnvZipkinURL = "zipkin_url"
)

type Server struct {
	Router    *gin.Engine
	Agent     agent.Agent
	Datastore models.Datastore
	MQ        models.MessageQueue
	LogDB     models.LogStore

	appListeners []extenders.AppListener
	middlewares  []Middleware
}

// NewFromEnv creates a new Functions server based on env vars.
func NewFromEnv(ctx context.Context, opts ...ServerOption) *Server {
	ds, err := datastore.New(viper.GetString(EnvDBURL))
	if err != nil {
		logrus.WithError(err).Fatalln("Error initializing datastore.")
	}

	mq, err := mqs.New(viper.GetString(EnvMQURL))
	if err != nil {
		logrus.WithError(err).Fatal("Error initializing message queue.")
	}

	var logDB models.LogStore = ds
	if ldb := viper.GetString(EnvLOGDBURL); ldb != "" && ldb != viper.GetString(EnvDBURL) {
		logDB, err = logs.New(viper.GetString(EnvLOGDBURL))
		if err != nil {
			logrus.WithError(err).Fatal("Error initializing logs store.")
		}
	}

	return New(ctx, ds, mq, logDB, opts...)
}

// New creates a new Functions server with the passed in datastore, message queue and API URL
func New(ctx context.Context, ds models.Datastore, mq models.MessageQueue, logDB models.LogStore, opts ...ServerOption) *Server {
	s := &Server{
		Agent:     agent.New(cache.Wrap(ds), mq), // only add datastore caching to agent
		Router:    gin.New(),
		Datastore: ds,
		MQ:        mq,
		LogDB:     logDB,
	}

	setMachineId()
	setTracer()
	s.Router.Use(loggerWrap, traceWrap, panicWrap)
	s.bindHandlers(ctx)

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(s)
	}
	return s
}

// we should use http grr
func traceWrap(c *gin.Context) {
	// try to grab a span from the request if made from another service, ignore err if not
	wireContext, _ := opentracing.GlobalTracer().Extract(
		opentracing.HTTPHeaders,
		opentracing.HTTPHeadersCarrier(c.Request.Header))

	// Create the span referring to the RPC client if available.
	// If wireContext == nil, a root span will be created.
	// TODO we should add more tags?
	serverSpan := opentracing.StartSpan("serve_http", ext.RPCServerOption(wireContext), opentracing.Tag{Key: "path", Value: c.Request.URL.Path})
	defer serverSpan.Finish()

	ctx := opentracing.ContextWithSpan(c.Request.Context(), serverSpan)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func setTracer() {
	var (
		debugMode          = false
		serviceName        = "fn-server"
		serviceHostPort    = "localhost:8080" // meh
		zipkinHTTPEndpoint = viper.GetString(EnvZipkinURL)
		// ex: "http://zipkin:9411/api/v1/spans"
	)

	if zipkinHTTPEndpoint == "" {
		return
	}

	logger := zipkintracer.LoggerFunc(func(i ...interface{}) error { logrus.Error(i...); return nil })

	collector, err := zipkintracer.NewHTTPCollector(zipkinHTTPEndpoint, zipkintracer.HTTPLogger(logger))
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't start trace collector")
	}
	tracer, err := zipkintracer.NewTracer(zipkintracer.NewRecorder(collector, debugMode, serviceHostPort, serviceName),
		zipkintracer.ClientServerSameSpan(true),
		zipkintracer.TraceID128Bit(true),
	)
	if err != nil {
		logrus.WithError(err).Fatalln("couldn't start tracer")
	}

	opentracing.SetGlobalTracer(tracer)
	logrus.WithFields(logrus.Fields{"url": zipkinHTTPEndpoint}).Info("started tracer")
}

func setMachineId() {
	port := uint16(viper.GetInt(EnvPort))
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

func panicWrap(c *gin.Context) {
	defer func(c *gin.Context) {
		if rec := recover(); rec != nil {
			err, ok := rec.(error)
			if !ok {
				err = fmt.Errorf("fn: %v", rec)
			}
			handleErrorResponse(c, err)
			c.Abort()
		}
	}(c)
	c.Next()
}

func loggerWrap(c *gin.Context) {
	ctx, _ := common.LoggerWithFields(c.Request.Context(), extractFields(c))

	if appName := c.Param(api.CApp); appName != "" {
		c.Set(api.AppName, appName)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), api.AppName, appName))
	}

	if routePath := c.Param(api.CRoute); routePath != "" {
		c.Set(api.Path, routePath)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), api.Path, routePath))
	}

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func appWrap(c *gin.Context) {
	appName := c.GetString(api.AppName)
	if appName == "" {
		handleErrorResponse(c, models.ErrAppsMissingName)
		c.Abort()
		return
	}
	c.Next()
}

func (s *Server) handleRunnerRequest(c *gin.Context) {
	s.handleRequest(c)
}

func extractFields(c *gin.Context) logrus.Fields {
	fields := logrus.Fields{"action": path.Base(c.HandlerName())}
	for _, param := range c.Params {
		fields[param.Key] = param.Value
	}
	return fields
}

func (s *Server) Start(ctx context.Context) {
	ctx = contextWithSignal(ctx, os.Interrupt)
	s.startGears(ctx)
}

func (s *Server) startGears(ctx context.Context) {
	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	listen := fmt.Sprintf(":%d", viper.GetInt(EnvPort))

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
		<-ctx.Done()                          // listening for signals...
		server.Shutdown(context.Background()) // we can wait
	}()

	err := server.ListenAndServe()
	if err != nil {
		logrus.WithError(err).Error("error opening server")
	}

	s.Agent.Close() // after we stop taking requests, wait for all tasks to finish
}

func (s *Server) bindHandlers(ctx context.Context) {
	engine := s.Router

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)
	engine.GET("/stats", s.handleStats)
	engine.GET("/metrics", s.handlePrometheusMetrics)

	{
		v1 := engine.Group("/v1")
		v1.Use(s.middlewareWrapperFunc(ctx))
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
			apps.GET("/routes/*route", s.handleRouteGet)
			apps.PATCH("/routes/*route", s.handleRoutesPostPutPatch)
			apps.PUT("/routes/*route", s.handleRoutesPostPutPatch)
			apps.DELETE("/routes/*route", s.handleRouteDelete)

			apps.GET("/calls", s.handleCallList)

			apps.GET("/calls/:call", s.handleCallGet)
			apps.GET("/calls/:call/log", s.handleCallLogGet)
			apps.DELETE("/calls/:call/log", s.handleCallLogDelete)
		}
	}

	{
		runner := engine.Group("/r")
		runner.Use(appWrap)
		runner.Any("/:app", s.handleRunnerRequest)
		runner.Any("/:app/*route", s.handleRunnerRequest)
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
