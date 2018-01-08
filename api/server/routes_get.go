package server

import (
	"net/http"
	"path"

	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleRouteGet(c *gin.Context) {
	ctx := c.Request.Context()

	appIDName := c.MustGet(api.App).(string)
	routePath := path.Clean("/" + c.MustGet(api.Path).(string))
	route, err := s.datastore.GetRoute(ctx, appIDName, routePath)
	if err != nil {
		handleErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
