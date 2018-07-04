package server

import (
	"github.com/fnproject/fn/api"
	"github.com/gin-gonic/gin"
	"net/http"
	"path"
)

func (s *Server) handleRouteGetAPI(c *gin.Context) {
	ctx := c.Request.Context()

	routePath := path.Clean("/" + c.MustGet(api.Path).(string))
	route, err := s.datastore.GetRoute(ctx, c.MustGet(api.AppID).(string), routePath)
	if err != nil {
		handleV1ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusOK, routeResponse{"Successfully loaded route", route})
}
