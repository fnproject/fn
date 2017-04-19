package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/ccirello/supervisor"
	"github.com/gin-gonic/gin"
	"github.com/kumokit/functions/api"
	"github.com/kumokit/functions/api/datastore"
	"github.com/kumokit/functions/api/models"
	"github.com/kumokit/functions/api/mqs"
	"github.com/kumokit/functions/api/runner"
	"github.com/kumokit/functions/api/runner/task"
	"github.com/kumokit/functions/api/server/internal/routecache"
	"github.com/kumokit/runner/common"
	"github.com/spf13/viper"
)

const (
	EnvLogLevel = "log_level"
	EnvMQURL    = "mq_url"
	EnvDBURL    = "db_url"
	EnvPort     = "port" // be careful, Gin expects this variable to be "port"
	EnvAPIURL   = "api_url"
)

type Server struct {
	Datastore models.Datastore
	Runner    *runner.Runner
	Router    *gin.Engine
	MQ        models.MessageQueue
	Enqueue   models.Enqueue

	apiURL string

	specialHandlers []SpecialHandler
	appListeners    []AppListener
	middlewares     []Middleware
	runnerListeners []RunnerListener

	mu           sync.Mutex // protects hotroutes
	hotroutes    *routecache.Cache
	tasks        chan task.Request
	singleflight singleflight // singleflight assists Datastore
}

const cacheSize = 1024

// NewFromEnv creates a new IronFunctions server based on env vars.
func NewFromEnv(ctx context.Context) *Server {
	ds, err := datastore.New(viper.GetString(EnvDBURL))
	if err != nil {
		logrus.WithError(err).Fatalln("Error initializing datastore.")
	}

	mq, err := mqs.New(viper.GetString(EnvMQURL))
	if err != nil {
		logrus.WithError(err).Fatal("Error initializing message queue.")
	}

	apiURL := viper.GetString(EnvAPIURL)

	return New(ctx, ds, mq, apiURL)
}

// New creates a new IronFunctions server with the passed in datastore, message queue and API URL
func New(ctx context.Context, ds models.Datastore, mq models.MessageQueue, apiURL string, opts ...ServerOption) *Server {
	metricLogger := runner.NewMetricLogger()
	funcLogger := runner.NewFuncLogger()

	rnr, err := runner.New(ctx, funcLogger, metricLogger)
	if err != nil {
		logrus.WithError(err).Fatalln("Failed to create a runner")
		return nil
	}

	tasks := make(chan task.Request)
	s := &Server{
		Runner:    rnr,
		Router:    gin.New(),
		Datastore: ds,
		MQ:        mq,
		hotroutes: routecache.New(cacheSize),
		tasks:     tasks,
		Enqueue:   DefaultEnqueue,
		apiURL:    apiURL,
	}

	s.Router.Use(prepareMiddleware(ctx))
	s.bindHandlers(ctx)

	for _, opt := range opts {
		opt(s)
	}
	return s
}

// todo: remove this or change name
func prepareMiddleware(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(ctx, extractFields(c))

		if appName := c.Param(api.CApp); appName != "" {
			c.Set(api.AppName, appName)
		}

		if routePath := c.Param(api.CRoute); routePath != "" {
			c.Set(api.Path, routePath)
		}

		// todo: can probably replace the "ctx" value with the Go 1.7 context on the http.Request
		c.Set("ctx", ctx)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func DefaultEnqueue(ctx context.Context, mq models.MessageQueue, task *models.Task) (*models.Task, error) {
	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"call_id": task.ID})
	return mq.Push(ctx, task)
}

func (s *Server) cacheget(appname, path string) (*models.Route, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	route, ok := s.hotroutes.Get(appname, path)
	if !ok {
		return nil, false
	}
	return route, ok
}

func (s *Server) cacherefresh(route *models.Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hotroutes.Refresh(route)
}

func (s *Server) cachedelete(appname, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hotroutes.Delete(appname, path)
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
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
			return
		}
		c.JSON(http.StatusAccepted, task)
	case "DELETE":
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, err)
			return
		}
		var task models.Task
		if err = json.Unmarshal(body, &task); err != nil {
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, err)
			return
		}

		if err := s.MQ.Delete(ctx, &task); err != nil {
			logrus.WithError(err).Error()
			c.JSON(http.StatusInternalServerError, err)
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
	close(s.tasks)
}

func (s *Server) startGears(ctx context.Context) {
	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	listen := fmt.Sprintf(":%d", viper.GetInt(EnvPort))
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		logrus.WithError(err).Fatalln("Failed to serve functions API.")
	}
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
		runner.RunAsyncRunner(ctx, s.apiURL, s.tasks, s.Runner)
	})

	svr.AddFunc(func(ctx context.Context) {
		runner.StartWorkers(ctx, s.Runner, s.tasks)
	})

	svr.Serve(ctx)
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

		apps := v1.Group("/apps/:app")
		{
			apps.GET("/routes", s.handleRouteList)
			apps.POST("/routes", s.handleRouteCreate)
			apps.GET("/routes/*route", s.handleRouteGet)
			apps.PATCH("/routes/*route", s.handleRouteUpdate)
			apps.DELETE("/routes/*route", s.handleRouteDelete)
		}
	}

	engine.DELETE("/tasks", s.handleTaskRequest)
	engine.GET("/tasks", s.handleTaskRequest)
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
