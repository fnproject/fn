package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/ccirello/supervisor"
	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/datastore"
	"github.com/fnproject/fn/api/id"
	"github.com/fnproject/fn/api/logs"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/mqs"
	"github.com/fnproject/fn/api/runner"
	"github.com/fnproject/fn/api/runner/common"
	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/patrickmn/go-cache"
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
	Datastore models.Datastore
	Runner    *runner.Runner
	Router    *gin.Engine
	MQ        models.MessageQueue
	Enqueue   models.Enqueue
	LogDB     models.FnLog

	apiURL string

	specialHandlers []SpecialHandler
	appListeners    []AppListener
	middlewares     []Middleware
	runnerListeners []RunnerListener

	routeCache   *cache.Cache
	singleflight singleflight // singleflight assists Datastore
}

const cacheSize = 1024

// NewFromEnv creates a new Functions server based on env vars.
func NewFromEnv(ctx context.Context) *Server {
	ds, err := datastore.New(viper.GetString(EnvDBURL))
	if err != nil {
		logrus.WithError(err).Fatalln("Error initializing datastore.")
	}

	mq, err := mqs.New(viper.GetString(EnvMQURL))
	if err != nil {
		logrus.WithError(err).Fatal("Error initializing message queue.")
	}

	var logDB models.FnLog = ds
	if ldb := viper.GetString(EnvLOGDBURL); ldb != "" && ldb != viper.GetString(EnvDBURL) {
		logDB, err = logs.New(viper.GetString(EnvLOGDBURL))
		if err != nil {
			logrus.WithError(err).Fatal("Error initializing logs store.")
		}
	}

	apiURL := viper.GetString(EnvAPIURL)

	return New(ctx, ds, mq, logDB, apiURL)
}

// New creates a new Functions server with the passed in datastore, message queue and API URL
func New(ctx context.Context, ds models.Datastore, mq models.MessageQueue, logDB models.FnLog, apiURL string, opts ...ServerOption) *Server {
	funcLogger := runner.NewFuncLogger(logDB)

	rnr, err := runner.New(ctx, funcLogger, ds)
	if err != nil {
		logrus.WithError(err).Fatalln("Failed to create a runner")
		return nil
	}

	s := &Server{
		Runner:     rnr,
		Router:     gin.New(),
		Datastore:  ds,
		MQ:         mq,
		routeCache: cache.New(5*time.Second, 5*time.Minute),
		LogDB:      logDB,
		Enqueue:    DefaultEnqueue,
		apiURL:     apiURL,
	}

	setMachineId()
	setTracer()
	s.Router.Use(loggerWrap, traceWrap)
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
	serverSpan := opentracing.StartSpan("serve_http", ext.RPCServerOption(wireContext), opentracing.Tag{"path", c.Request.URL.Path})
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

func loggerWrap(c *gin.Context) {
	ctx, _ := common.LoggerWithFields(c.Request.Context(), extractFields(c))

	if appName := c.Param(api.CApp); appName != "" {
		c.Set(api.AppName, appName)
	}

	if routePath := c.Param(api.CRoute); routePath != "" {
		c.Set(api.Path, routePath)
	}

	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func DefaultEnqueue(ctx context.Context, mq models.MessageQueue, task *models.Task) (*models.Task, error) {
	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"call_id": task.ID})
	return mq.Push(ctx, task)
}

func routeCacheKey(appname, path string) string {
	return fmt.Sprintf("%s_%s", appname, path)
}
func (s *Server) cacheget(appname, path string) (*models.Route, bool) {
	route, ok := s.routeCache.Get(routeCacheKey(appname, path))
	if !ok {
		return nil, false
	}
	return route.(*models.Route), ok
}

func (s *Server) cachedelete(appname, path string) {
	s.routeCache.Delete(routeCacheKey(appname, path))
}

func (s *Server) handleRunnerRequest(c *gin.Context) {
	s.handleRequest(c, s.Enqueue)
}

func (s *Server) handleTaskRequest(c *gin.Context) {
	ctx, _ := common.LoggerWithFields(c, nil)
	switch c.Request.Method {
	case "GET":
		task, err := s.MQ.Reserve(ctx)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}
		c.JSON(http.StatusOK, task)
	case "DELETE":
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			handleErrorResponse(c, err)
			return
		}
		var task models.Task
		if err = json.Unmarshal(body, &task); err != nil {
			handleErrorResponse(c, err)
			return
		}

		if err := s.MQ.Delete(ctx, &task); err != nil {
			handleErrorResponse(c, err)
			return
		}
		c.JSON(http.StatusAccepted, task)
	}
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
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		logrus.WithError(err).Fatalln("Failed to serve functions API.")
	}

	const runHeader = `
	     ____                  __
	    / __ \_________ ______/ /__
	   / / / / ___/ __ / ___/ / _  \
	  / /_/ / /  / /_/ / /__/ /  __/
	  \_________ \__,_/\___/_/\____
	     / ____/_  __ ___  _____/ /_( )___  ____  _____
	    / /_  / / / / __ \/ ___/ __/ / __ \/ __ \/ ___/
	   / __/ / /_/ / / / / /__/ /_/ / /_/ / / / (__  )
	  /_/    \____/_/ /_/\___/\__/_/\____/_/ /_/____/
	`
	fmt.Println(runHeader)
	logrus.Infof("Serving Functions API on address `%s`", listen)

	svr := &supervisor.Supervisor{
		MaxRestarts: supervisor.AlwaysRestart,
		Log: func(msg interface{}) {
			logrus.Debug("supervisor: ", msg)
		},
	}

	svr.AddFunc(func(ctx context.Context) {
		go func() {
			err := http.Serve(listener, s.Router)
			if err != nil {
				logrus.Fatalf("Error serving API: %v", err)
			}
		}()
		<-ctx.Done()
	})

	svr.AddFunc(func(ctx context.Context) {
		runner.RunAsyncRunner(ctx, s.apiURL, s.Runner, s.Datastore)
	})

	svr.Serve(ctx)
	s.Runner.Wait() // wait for tasks to finish (safe shutdown)
}

func (s *Server) bindHandlers(ctx context.Context) {
	engine := s.Router

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)
	engine.GET("/stats", s.handleStats)

	v1 := engine.Group("/v1")
	v1.Use(s.middlewareWrapperFunc(ctx))
	{
		v1.GET("/apps", s.handleAppList)
		v1.POST("/apps", s.handleAppCreate)

		v1.GET("/apps/:app", s.handleAppGet)
		v1.PATCH("/apps/:app", s.handleAppUpdate)
		v1.DELETE("/apps/:app", s.handleAppDelete)

		v1.GET("/routes", s.handleRouteList)

		v1.GET("/calls/:call", s.handleCallGet)
		v1.GET("/calls/:call/log", s.handleCallLogGet)
		v1.DELETE("/calls/:call/log", s.handleCallLogDelete)

		apps := v1.Group("/apps/:app")
		{
			apps.GET("/routes", s.handleRouteList)
			apps.POST("/routes", s.handleRouteCreateOrUpdate)
			apps.GET("/routes/*route", s.handleRouteGet)
			apps.PATCH("/routes/*route", s.handleRouteCreateOrUpdate)
			apps.PUT("/routes/*route", s.handleRouteCreateOrUpdate)
			apps.DELETE("/routes/*route", s.handleRouteDelete)
			apps.GET("/calls/*route", s.handleCallList)
		}
	}

	engine.DELETE("/tasks", s.handleTaskRequest)
	engine.GET("/tasks", s.handleTaskRequest)
	engine.Any("/r/:app", s.handleRunnerRequest)
	engine.Any("/r/:app/*route", s.handleRunnerRequest)

	// This final route is used for extensions, see Server.Add
	engine.NoRoute(s.handleSpecial)
}

type appResponse struct {
	Message string      `json:"message"`
	App     *models.App `json:"app"`
}

type appsResponse struct {
	Message string      `json:"message"`
	Apps    models.Apps `json:"apps"`
}

type routeResponse struct {
	Message string        `json:"message"`
	Route   *models.Route `json:"route"`
}

type routesResponse struct {
	Message string        `json:"message"`
	Routes  models.Routes `json:"routes"`
}

type tasksResponse struct {
	Message string      `json:"message"`
	Task    models.Task `json:"tasksResponse"`
}

type fnCallResponse struct {
	Message string         `json:"message"`
	Call    *models.FnCall `json:"call"`
}

type fnCallsResponse struct {
	Message string         `json:"message"`
	Calls   models.FnCalls `json:"calls"`
}

type fnCallLogResponse struct {
	Message string            `json:"message"`
	Log     *models.FnCallLog `json:"log"`
}
