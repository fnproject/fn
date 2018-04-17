package server

import (
	"net/http"
	"path"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func routeGet(s *Server, appID string, c *gin.Context) {
	ctx := c.Request.Context()

	routePath := path.Clean("/" + c.MustGet(api.Path).(string))
	route, err := s.datastore.GetRoute(ctx, appID, routePath)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}

func (s *Server) handleRouteGetAPI(c *gin.Context) {
	routeGet(s, c.MustGet(api.AppID).(string), c)
}

func (s *Server) handleRouteGetRunner(c *gin.Context) {
	routeGet(s, c.Param(api.CApp), c)
}
