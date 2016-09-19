package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/ifaces"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	titancommon "github.com/iron-io/worker/common"
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
		Router:    gin.Default(),
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
		return s.MQ.Push(task)
	}
	handleRequest(c, enqueue)
}

func (s *Server) handleTaskRequest(c *gin.Context, del bool) {
	if !del {
		task, err := s.MQ.Reserve()
		if err != nil {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, simpleError(models.ErrRoutesList))
			return
		}
		c.JSON(http.StatusAccepted, task)
	} else {
		body, err := ioutil.ReadAll(c.Request.Body)
		if err != nil {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, err)
			return
		}

		bodyStr := strings.TrimSpace(string(body))
		if bodyStr == "null" {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, err)
			return
		}

		var task models.Task
		if err = json.Unmarshal(body, &task); err != nil {
			logrus.WithError(err)
			c.JSON(http.StatusInternalServerError, err)
		}

		if err := s.MQ.Delete(&task); err != nil {
			logrus.WithError(err)
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
		ctx, _ := titancommon.LoggerWithFields(ctx, extractFields(c))
		c.Set("ctx", ctx)
		c.Next()
	})

	bindHandlers(s.Router, s.handleRunnerRequest, s.handleTaskRequest)

	// By default it serves on :8080 unless a
	// PORT environment variable was defined.
	s.Router.Run()
}

func bindHandlers(engine *gin.Engine, reqHandler func(ginC *gin.Context), taskHandler func(ginC *gin.Context, del bool)) {
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

	taskHandlerDelete := func(ginC *gin.Context) {
		taskHandler(ginC, true)
	}
	taskHandlerReserve := func(ginC *gin.Context) {
		taskHandler(ginC, false)
	}

	engine.GET("/tasks", taskHandlerReserve)
	engine.DELETE("/tasks", taskHandlerDelete)
	engine.Any("/r/:app/*route", reqHandler)

	// This final route is used for extensions, see Server.Add
	engine.NoRoute(handleSpecial)
}

func simpleError(err error) *models.Error {
	return &models.Error{&models.ErrorBody{Message: err.Error()}}
}
