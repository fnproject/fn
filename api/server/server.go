package server

import (
	"path"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/ifaces"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/runner"
	titancommon "github.com/iron-io/titan/common"
)

// Would be nice to not have this is a global, but hard to pass things around to the
// handlers in Gin without it.
var Api *Server

type Server struct {
	Runner          *runner.Runner
	Router          *gin.Engine
	Config          *models.Config
	Datastore       models.Datastore
	AppListeners    []ifaces.AppListener
	SpecialHandlers []ifaces.SpecialHandler
}

func New(c *models.Config, ds models.Datastore, r *runner.Runner) *Server {
	Api = &Server{
		Router:    gin.Default(),
		Config:    c,
		Datastore: ds,
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
	handleRunner(ginC)
	return nil
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

	bindHandlers(s.Router)

	// Default to :8080
	s.Router.Run()
}

func bindHandlers(engine *gin.Engine) {
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

	engine.Any("/r/:app/*route", handleRunner)

	// This final route is used for extensions, see Server.Add
	engine.NoRoute(handleSpecial)
}

func simpleError(err error) *models.Error {
	return &models.Error{&models.ErrorBody{Message: err.Error()}}
}
