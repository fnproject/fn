package server

import (
	"net/http"
	"path"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleRouteGet(c *gin.Context) {
	ctx := c.Request.Context()

	appName := c.MustGet(api.AppName).(string)
	routePath := path.Clean("/" + c.MustGet(api.Path).(string))
	route, err := s.Datastore.GetRoute(ctx, appName, routePath)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
