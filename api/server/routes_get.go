package server

import (
	"context"
	"net/http"
	"path"

	"github.com/gin-gonic/gin"
	"gitlab-odx.oracle.com/odx/functions/api"
)

func (s *Server) handleRouteGet(c *gin.Context) {
	ctx := c.MustGet("ctx").(context.Context)

	appName := c.MustGet(api.AppName).(string)
	routePath := path.Clean(c.MustGet(api.Path).(string))

	route, err := s.Datastore.GetRoute(ctx, appName, routePath)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
