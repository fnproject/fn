package server

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/ifaces"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	"github.com/iron-io/runner/common"
)

// Would be nice to not have this is a global, but hard to pass things around to the
// handlers in Gin without it.
var Api *Server

type Server struct {
	Runner          *runner.Runner
	Router          *gin.Engine
	Datastore       models.Datastore
	MQ              models.MessageQueue
	AppListeners    []ifaces.AppListener
	SpecialHandlers []ifaces.SpecialHandler
}

func New(ds models.Datastore, mq models.MessageQueue, r *runner.Runner) *Server {
	Api = &Server{
		Router:    gin.New(),
		Datastore: ds,
		MQ:        mq,
		Runner:    r,
	}
	return Api
}

// AddAppListener adds a listener that will be notified on App changes.
func (s *Server) AddAppListener(listener ifaces.AppListener) {
	s.AppListeners = append(s.AppListeners, listener)
}

func (s *Server) FireBeforeAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppListeners {
		err := l.BeforeAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) FireAfterAppUpdate(ctx context.Context, app *models.App) error {
	for _, l := range s.AppListeners {
		err := l.AfterAppUpdate(ctx, app)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) AddSpecialHandler(handler ifaces.SpecialHandler) {
	s.SpecialHandlers = append(s.SpecialHandlers, handler)
}

func (s *Server) UseSpecialHandlers(ginC *gin.Context) error {
	c := &SpecialHandlerContext{
		server:     s,
		ginContext: ginC,
	}
	for _, l := range s.SpecialHandlers {
		err := l.Handle(c)
		if err != nil {
			return err
		}
	}
	// now call the normal runner call
	handleRequest(ginC, nil)
	return nil
}

func (s *Server) handleRunnerRequest(c *gin.Context) {
	enqueue := func(task *models.Task) (*models.Task, error) {
		c.JSON(http.StatusAccepted, map[string]string{"call_id": task.ID})
		ctx, _ := common.LoggerWithFields(c, logrus.Fields{"call_id": task.ID})
		return s.MQ.Push(ctx, task)
	}
	handleRequest(c, enqueue)
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

func (s *Server) Run(ctx context.Context) {
	s.Router.Use(func(c *gin.Context) {
		ctx, _ := common.LoggerWithFields(ctx, extractFields(c))
		c.Set("ctx", ctx)
		c.Next()
	})

	bindHandlers(s.Router, s.handleRunnerRequest, s.handleTaskRequest)

	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	s.Router.Run()
}

func bindHandlers(engine *gin.Engine, reqHandler func(ginC *gin.Context), taskHandler func(ginC *gin.Context)) {
	engine.Use(gin.Logger())

	engine.GET("/", handlePing)
	engine.GET("/version", handleVersion)

	v1 := engine.Group("/v1")
	{
		v1.GET("/apps", handleAppList)
		v1.POST("/apps", handleAppCreate)

		v1.GET("/apps/:app", handleAppGet)
		v1.PUT("/apps/:app", handleAppUpdate)
		v1.DELETE("/apps/:app", handleAppDelete)

		v1.GET("/routes", handleRouteList)

		apps := v1.Group("/apps/:app")
		{
			apps.GET("/routes", handleRouteList)
			apps.POST("/routes", handleRouteCreate)
			apps.GET("/routes/*route", handleRouteGet)
			apps.PUT("/routes/*route", handleRouteUpdate)
			apps.DELETE("/routes/*route", handleRouteDelete)
		}
	}

	engine.DELETE("/tasks", taskHandler)
	engine.GET("/tasks", taskHandler)
	engine.Any("/r/:app/*route", reqHandler)

	// This final route is used for extensions, see Server.Add
	engine.NoRoute(handleSpecial)
}

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
