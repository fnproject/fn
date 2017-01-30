// TODO: it would be nice to move these into the top level folder so people can use these with the "functions" package, eg: functions.ApiHandler
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iron-io/functions/api"
	"github.com/iron-io/functions/api/models"
)

type ApiHandlerFunc func(w http.ResponseWriter, r *http.Request)

// ServeHTTP calls f(w, r).
func (f ApiHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f(w, r)
}

type ApiHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type ApiAppHandler interface {
	// Handle(ctx context.Context)
	ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App)
}

type ApiAppHandlerFunc func(w http.ResponseWriter, r *http.Request, app *models.App)

// ServeHTTP calls f(w, r).
func (f ApiAppHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App) {
	f(w, r, app)
}

func (s *Server) apiHandlerWrapperFunc(apiHandler ApiHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiHandler.ServeHTTP(c.Writer, c.Request)
	}
}

func (s *Server) apiAppHandlerWrapperFunc(apiHandler ApiAppHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// get the app
		appName := c.Param(api.CApp)
		app, err := s.Datastore.GetApp(c.Request.Context(), appName)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if app == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		apiHandler.ServeHTTP(c.Writer, c.Request, app)
	}
}

// AddEndpoint adds an endpoint to /v1/x
func (s *Server) AddEndpoint(method, path string, handler ApiHandler) {
	v1 := s.Router.Group("/v1")
	// v1.GET("/apps/:app/log", logHandler(cfg))
	v1.Handle(method, path, s.apiHandlerWrapperFunc(handler))
}

// AddEndpoint adds an endpoint to /v1/x
func (s *Server) AddEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request)) {
	s.AddEndpoint(method, path, ApiHandlerFunc(handler))
}

// AddAppEndpoint adds an endpoints to /v1/apps/:app/x
func (s *Server) AddAppEndpoint(method, path string, handler ApiAppHandler) {
	v1 := s.Router.Group("/v1")
	v1.Handle(method, "/apps/:app"+path, s.apiAppHandlerWrapperFunc(handler))
}

// AddAppEndpoint adds an endpoints to /v1/apps/:app/x
func (s *Server) AddAppEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request, app *models.App)) {
	s.AddAppEndpoint(method, path, ApiAppHandlerFunc(handler))
}
