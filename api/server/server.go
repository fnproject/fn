package server

import (
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
)

var Api *Server

type Server struct {
	Router    *gin.Engine
	Config    *models.Config
	Datastore models.Datastore
}

func New(ds models.Datastore, config *models.Config) *Server {
	Api = &Server{
		Router:    gin.Default(),
		Config:    config,
		Datastore: ds,
	}
	return Api
}

func extractFields(c *gin.Context) logrus.Fields {
	fields := logrus.Fields{"action": path.Base(c.HandlerName())}
	for _, param := range c.Params {
		fields[param.Key] = param.Value
	}
	return fields
}

func (s *Server) Run() {
	s.Router.Use(func(c *gin.Context) {
		c.Set("log", logrus.WithFields(extractFields(c)))
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

		apps := v1.Group("/apps/:app")
		{
			apps.GET("/routes", handleRouteList)
			apps.POST("/routes", handleRouteCreate)
			apps.GET("/routes/:route", handleRouteGet)
			apps.PUT("/routes/:route", handleRouteUpdate)
			apps.DELETE("/routes/:route", handleRouteDelete)
		}

	}

	engine.Any("/r/:app/*route", handleRunner)
	engine.NoRoute(handleRunner)
}

func simpleError(err error) *models.Error {
	return &models.Error{&models.ErrorBody{Message: err.Error()}}
}
