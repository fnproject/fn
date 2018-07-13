package server

// TODO: it would be nice to move these into the top level folder so people can use these with the "functions" package, eg: functions.ApiHandler

import (
	"net/http"

	"github.com/fnproject/fn/api"
	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/fnext"
	"github.com/gin-gonic/gin"
)

func (s *Server) apiHandlerWrapperFn(apiHandler fnext.APIHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiHandler.ServeHTTP(c.Writer, c.Request)
	}
}

func (s *Server) apiAppHandlerWrapperFn(apiHandler fnext.APIAppHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		// get the app
		appID := c.MustGet(api.AppID).(string)
		app, err := s.datastore.GetAppByID(c.Request.Context(), appID)
		if err != nil {
			handleV1ErrorResponse(c, err)
			c.Abort()
			return
		}
		if app == nil {
			handleV1ErrorResponse(c, models.ErrAppsNotFound)
			c.Abort()
			return
		}

		apiHandler.ServeHTTP(c.Writer, c.Request, app)
	}
}

func (s *Server) apiRouteHandlerWrapperFn(apiHandler fnext.APIRouteHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		context := c.Request.Context()
		appID := c.MustGet(api.AppID).(string)
		routePath := "/" + c.Param(api.ParamRouteName)
		route, err := s.datastore.GetRoute(context, appID, routePath)
		if err != nil {
			handleV1ErrorResponse(c, err)
			c.Abort()
			return
		}
		if route == nil {
			handleV1ErrorResponse(c, models.ErrRoutesNotFound)
			c.Abort()
			return
		}

		app, err := s.datastore.GetAppByID(context, appID)
		if err != nil {
			handleV1ErrorResponse(c, err)
			c.Abort()
			return
		}
		if app == nil {
			handleV1ErrorResponse(c, models.ErrAppsNotFound)
			c.Abort()
			return
		}

		apiHandler.ServeHTTP(c.Writer, c.Request, app, route)
	}
}

// AddEndpoint adds an endpoint to /v1/x
func (s *Server) AddEndpoint(method, path string, handler fnext.APIHandler) {
	v1 := s.Router.Group("/v1")
	// v1.GET("/apps/:app/log", logHandler(cfg))
	v1.Handle(method, path, s.apiHandlerWrapperFn(handler))
}

// AddEndpointFunc adds an endpoint to /v1/x
func (s *Server) AddEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request)) {
	s.AddEndpoint(method, path, fnext.APIHandlerFunc(handler))
}

// AddAppEndpoint adds an endpoints to /v1/apps/:app/x
func (s *Server) AddAppEndpoint(method, path string, handler fnext.APIAppHandler) {
	v1 := s.Router.Group("/v1")
	v1.Use(s.checkAppPresenceByName())
	v1.Handle(method, "/apps/:app"+path, s.apiAppHandlerWrapperFn(handler))
}

// AddAppEndpointFunc adds an endpoints to /v1/apps/:app/x
func (s *Server) AddAppEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request, app *models.App)) {
	s.AddAppEndpoint(method, path, fnext.APIAppHandlerFunc(handler))
}

// AddRouteEndpoint adds an endpoints to /v1/apps/:app/routes/:route/x
func (s *Server) AddRouteEndpoint(method, path string, handler fnext.APIRouteHandler) {
	v1 := s.Router.Group("/v1")
	v1.Use(s.checkAppPresenceByName())
	v1.Handle(method, "/apps/:app/routes/:route"+path, s.apiRouteHandlerWrapperFn(handler)) // conflicts with existing wildcard
}

// AddRouteEndpointFunc adds an endpoints to /v1/apps/:app/routes/:route/x
func (s *Server) AddRouteEndpointFunc(method, path string, handler func(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route)) {
	s.AddRouteEndpoint(method, path, fnext.APIRouteHandlerFunc(handler))
}
