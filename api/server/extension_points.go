// TODO: it would be nice to move these into the top level folder so people can use these with the "functions" package, eg: functions.ApiHandler
package server

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
)

func (s *Server) apiHandlerWrapperFunc(apiHandler fnext.ApiHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiHandler.ServeHTTP(c.Writer, c.Request)
	}
}

func (s *Server) apiAppHandlerWrapperFunc(apiHandler fnext.ApiAppHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// get the app
		appIDorName := c.Param(api.CApp)
		app, err := s.datastore.GetApp(c.Request.Context(),
			&models.App{Name: appIDorName, ID: appIDorName})
		if err != nil {
			handleErrorResponse(c, err)
			c.Abort()
			return
		}
		if app == nil {
			handleErrorResponse(c, models.ErrAppsNotFound)
			c.Abort()
			return
		}

		apiHandler.ServeHTTP(c.Writer, c.Request, app)
	}
}

func (s *Server) apiRouteHandlerWrapperFunc(apiHandler fnext.ApiRouteHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		context := c.Request.Context()
		// get the app
		appIDorName := c.Param(api.CApp)
		app := &models.App{Name: appIDorName, ID: appIDorName}
		app, err := s.datastore.GetApp(context, app)
		if err != nil {
			handleErrorResponse(c, err)
			c.Abort()
			return
		}
		if app == nil {
			handleErrorResponse(c, models.ErrAppsNotFound)
			c.Abort()
			return
		}
		// get the route TODO
		routePath := "/" + c.Param(api.CRoute)
		route, err := s.datastore.GetRoute(context, app.ID, routePath)
		if err != nil {
			handleErrorResponse(c, err)
			c.Abort()
			return
		}
		if route == nil {
			handleErrorResponse(c, models.ErrRoutesNotFound)
			c.Abort()
			return
		}

		apiHandler.ServeHTTP(c.Writer, c.Request, app, route)
	}
}

// AddEndpoint adds an endpoint to /v1/x
func (s *Server) AddEndpoint(method, path string, handler fnext.ApiHandler) {
	v1 := s.Router.Group("/v1")
	// v1.GET("/apps/:app/log", logHandler(cfg))
	v1.Handle(method, path, s.apiHandlerWrapperFunc(handler))
}

// AddEndpoint adds an endpoint to /v1/x
func (s *Server) AddEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request)) {
	s.AddEndpoint(method, path, fnext.ApiHandlerFunc(handler))
}

// AddAppEndpoint adds an endpoints to /v1/apps/:app/x
func (s *Server) AddAppEndpoint(method, path string, handler fnext.ApiAppHandler) {
	v1 := s.Router.Group("/v1")
	v1.Handle(method, "/apps/:app"+path, s.apiAppHandlerWrapperFunc(handler))
}

// AddAppEndpoint adds an endpoints to /v1/apps/:app/x
func (s *Server) AddAppEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request, app *models.App)) {
	s.AddAppEndpoint(method, path, fnext.ApiAppHandlerFunc(handler))
}

// AddRouteEndpoint adds an endpoints to /v1/apps/:app/routes/:route/x
func (s *Server) AddRouteEndpoint(method, path string, handler fnext.ApiRouteHandler) {
	v1 := s.Router.Group("/v1")
	v1.Handle(method, "/apps/:app/routes/:route"+path, s.apiRouteHandlerWrapperFunc(handler)) // conflicts with existing wildcard
}

// AddRouteEndpoint adds an endpoints to /v1/apps/:app/routes/:route/x
func (s *Server) AddRouteEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route)) {
	s.AddRouteEndpoint(method, path, fnext.ApiRouteHandlerFunc(handler))
}
