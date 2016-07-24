package server

import (
	"fmt"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/server/datastore"
	"github.com/iron-io/functions/api/server/router"
)

type Server struct {
	router *gin.Engine
	cfg    *models.Config
}

func New(config *models.Config) *Server {
	return &Server{
		router: gin.Default(),
		cfg:    config,
	}
}

func extractFields(c *gin.Context) logrus.Fields {
	fields := logrus.Fields{"action": path.Base(c.HandlerName())}
	for _, param := range c.Params {
		fields[param.Key] = param.Value
	}
	return fields
}

func (s *Server) Start() {
	if s.cfg.DatabaseURL == "" {
		cwd, _ := os.Getwd()
		s.cfg.DatabaseURL = fmt.Sprintf("bolt://%s/bolt.db?bucket=funcs", cwd)
	}

	if s.cfg.API == "" {
		s.cfg.API = "http://localhost:8080"
	}

	ds, err := datastore.New(s.cfg.DatabaseURL)
	if err != nil {
		logrus.WithError(err).Fatalln("Invalid DB url.")
	}

	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.DebugLevel)

	s.router.Use(func(c *gin.Context) {
		c.Set("config", s.cfg)
		c.Set("store", ds)
		c.Set("log", logrus.WithFields(extractFields(c)))
		c.Next()
	})

	router.Start(s.router)

	// Default to :8080
	s.router.Run()
}
