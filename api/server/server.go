package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/functions/api/runner/task"
	"github.com/iron-io/runner/common"
)

type Server struct {
	Datastore models.Datastore
	Runner    *runner.Runner
	Router    *gin.Engine
	MQ        models.MessageQueue
	Enqueue   models.Enqueue

	specialHandlers    []SpecialHandler
	appCreateListeners []AppCreateListener
	appUpdateListeners []AppUpdateListener
	appDeleteListeners []AppDeleteListener
	runnerListeners    []RunnerListener

	tasks        chan task.Request
	singleflight singleflight // singleflight assists Datastore
}

func New(ctx context.Context, ds models.Datastore, mq models.MessageQueue, r *runner.Runner, tasks chan task.Request, enqueue models.Enqueue, opts ...ServerOption) *Server {
	s := &Server{
		Runner:    r,
		Router:    gin.New(),
		Datastore: ds,
		MQ:        mq,
		tasks:     tasks,
		Enqueue:   enqueue,
	}

	s.Router.Use(prepareMiddleware(ctx))

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func prepareMiddleware(ctx context.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(ctx, extractFields(c))

		if appName := c.Param("app"); appName != "" {
			ctx = context.WithValue(ctx, "appName", appName)
		}

		if routePath := c.Param("route"); routePath != "" {
			ctx = context.WithValue(ctx, "routePath", routePath)
		}

		c.Set("ctx", ctx)
		c.Next()
	}
}

func DefaultEnqueue(ctx context.Context, mq models.MessageQueue, task *models.Task) (*models.Task, error) {
	ctx, _ = common.LoggerWithFields(ctx, logrus.Fields{"call_id": task.ID})
	return mq.Push(ctx, task)
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

func (s *Server) Run() {
	s.bindHandlers()

	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	go s.Router.Run()
}

func (s *Server) bindHandlers() {
	engine := s.Router

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)

	v1 := engine.Group("/v1")
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

var ErrInternalServerError = errors.New("Something unexpected happened on the server")

func simpleError(err error) *models.Error {
	return &models.Error{&models.ErrorBody{Message: err.Error()}}
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
